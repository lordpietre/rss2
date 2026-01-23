#!/usr/bin/env python3
"""
LLM Categorizer Worker - Categoriza noticias usando ExLlamaV2 (local)

Este worker:
1. Lee 10 noticias sin categorizar de la base de datos
2. Las envía a un LLM local (ExLlamaV2) para que determine la categoría
3. Actualiza la base de datos con las categorías asignadas

Modelo recomendado para RTX 3060 12GB:
- Mistral-7B-Instruct-v0.2 (GPTQ/AWQ/EXL2)
- OpenHermes-2.5-Mistral-7B
- Neural-Chat-7B
"""

import os
import sys
import time
import logging
import json
from typing import List, Dict, Optional
import psycopg2
from psycopg2.extras import execute_values

# Configuración de logging
logging.basicConfig(
    level=logging.INFO,
    format='[llm_categorizer] %(asctime)s %(levelname)s: %(message)s'
)
log = logging.getLogger(__name__)

# Configuración de base de datos
DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", ""),
}

# Configuración del worker
BATCH_SIZE = int(os.environ.get("LLM_BATCH_SIZE", 10))  # 10 noticias por lote
SLEEP_IDLE = int(os.environ.get("LLM_SLEEP_IDLE", 30))  # segundos
MODEL_PATH = os.environ.get("LLM_MODEL_PATH", "/app/models/llm")
GPU_SPLIT = os.environ.get("LLM_GPU_SPLIT", "auto")
MAX_SEQ_LEN = int(os.environ.get("LLM_MAX_SEQ_LEN", 4096))
CACHE_MODE = os.environ.get("LLM_CACHE_MODE", "FP16")

# Categorías predefinidas
CATEGORIES = [
    "Política",
    "Economía",
    "Tecnología",
    "Ciencia",
    "Salud",
    "Deportes",
    "Entretenimiento",
    "Internacional",
    "Nacional",
    "Sociedad",
    "Cultura",
    "Medio Ambiente",
    "Educación",
    "Seguridad",
    "Otros"
]

class ExLlamaV2Categorizer:
    """Wrapper para el modelo ExLlamaV2"""
    
    def __init__(self, model_path: str):
        """
        Inicializa el modelo ExLlamaV2
        
        Args:
            model_path: Ruta al modelo descargado (formato EXL2, GPTQ, etc.)
        """
        self.model_path = model_path
        self.model = None
        self.tokenizer = None
        self.cache = None
        self.generator = None
        
        log.info(f"Inicializando ExLlamaV2 desde: {model_path}")
        self._load_model()
    
    def _load_model(self):
        """Carga el modelo y componentes necesarios"""
        try:
            from exllamav2 import (
                ExLlamaV2,
                ExLlamaV2Config,
                ExLlamaV2Cache,
                ExLlamaV2Tokenizer,
            )
            from exllamav2.generator import (
                ExLlamaV2StreamingGenerator,
                ExLlamaV2Sampler
            )
            
            # Configuración del modelo
            config = ExLlamaV2Config()
            config.model_dir = self.model_path
            config.prepare()
            
            # Optimizaciones para RTX 3060 12GB
            config.max_seq_len = MAX_SEQ_LEN
            config.scale_pos_emb = 1.0
            config.scale_alpha_value = 1.0
            
            # Cargar modelo
            self.model = ExLlamaV2(config)
            
            log.info("Cargando modelo en GPU...")
            
            # Configurar GPU split (auto para single GPU)
            if GPU_SPLIT.lower() == "auto":
                self.model.load_autosplit(cache=None)
            else:
                split = [float(x.strip()) for x in GPU_SPLIT.split(",")]
                self.model.load(split)
            
            # Tokenizer
            self.tokenizer = ExLlamaV2Tokenizer(config)
            
            # Cache
            if CACHE_MODE == "FP16":
                self.cache = ExLlamaV2Cache(self.model, lazy=True)
            elif CACHE_MODE == "Q4":
                from exllamav2 import ExLlamaV2Cache_Q4
                self.cache = ExLlamaV2Cache_Q4(self.model, lazy=True)
            else:
                self.cache = ExLlamaV2Cache(self.model, lazy=True)
            
            # Generator
            self.generator = ExLlamaV2StreamingGenerator(
                self.model, 
                self.cache, 
                self.tokenizer
            )
            
            # Configuración de sampling
            self.settings = ExLlamaV2Sampler.Settings()
            self.settings.temperature = 0.1  # Determinista para clasificación
            self.settings.top_k = 10
            self.settings.top_p = 0.9
            self.settings.token_repetition_penalty = 1.05
            
            log.info("✓ Modelo cargado exitosamente")
            
        except ImportError as e:
            log.error(f"Error: ExLlamaV2 no está instalado. Instalar con: pip install exllamav2")
            log.error(f"Detalles: {e}")
            raise
        except Exception as e:
            log.error(f"Error cargando modelo: {e}")
            raise
    
    def categorize_news(self, news_items: List[Dict]) -> List[Dict]:
        """
        Categoriza un lote de noticias
        
        Args:
            news_items: Lista de diccionarios con 'id', 'titulo', 'resumen'
            
        Returns:
            Lista de diccionarios con 'id', 'categoria', 'confianza'
        """
        results = []
        
        for item in news_items:
            categoria, confianza = self._categorize_single(
                item['titulo'], 
                item['resumen']
            )
            
            results.append({
                'id': item['id'],
                'categoria': categoria,
                'confianza': confianza
            })
            
            log.info(f"Noticia {item['id']}: {categoria} (confianza: {confianza:.2f})")
        
        return results
    
    def _categorize_single(self, titulo: str, resumen: str) -> tuple:
        """
        Categoriza una noticia individual
        
        Returns:
            (categoria, confianza)
        """
        # Construir prompt
        prompt = self._build_prompt(titulo, resumen)
        
        # Generar respuesta
        try:
            self.generator.set_stop_conditions([self.tokenizer.eos_token_id])
            
            output = self.generator.generate_simple(
                prompt,
                self.settings,
                max_new_tokens=50,  # Solo necesitamos la categoría
                seed=1234
            )
            
            # Parsear respuesta
            categoria, confianza = self._parse_response(output)
            
            return categoria, confianza
            
        except Exception as e:
            log.error(f"Error durante la generación: {e}")
            return "Otros", 0.0
    
    def _build_prompt(self, titulo: str, resumen: str) -> str:
        """
        Construye el prompt para el LLM
        
        Usa el formato Mistral/ChatML
        """
        categories_str = ", ".join(CATEGORIES)
        
        # Prompt optimizado para clasificación
        prompt = f"""<s>[INST] Eres un asistente experto en clasificación de noticias.

Tu tarea es categorizar la siguiente noticia en UNA de estas categorías:
{categories_str}

Reglas:
1. Responde SOLO con el nombre de la categoría
2. Elige la categoría que MEJOR represente el contenido principal
3. Si no estás seguro, usa "Otros"

Noticia:
Título: {titulo}
Contenido: {resumen[:500]}

Categoría: [/INST]"""
        
        return prompt
    
    def _parse_response(self, output: str) -> tuple:
        """
        Parsea la respuesta del LLM
        
        Returns:
            (categoria, confianza)
        """
        # Limpiar respuesta
        response = output.strip()
        
        # Buscar la categoría en la respuesta
        for cat in CATEGORIES:
            if cat.lower() in response.lower():
                # Confianza simple basada en si es exacta
                confianza = 0.9 if cat in response else 0.7
                return cat, confianza
        
        # Si no se encuentra, usar "Otros"
        return "Otros", 0.5


def get_db_connection():
    """Obtiene conexión a la base de datos"""
    return psycopg2.connect(**DB_CONFIG)


def initialize_schema(conn):
    """
    Asegura que existan las tablas necesarias
    """
    log.info("Verificando esquema de base de datos...")
    
    with conn.cursor() as cur:
        # Agregar columnas si no existen
        cur.execute("""
            ALTER TABLE noticias 
            ADD COLUMN IF NOT EXISTS llm_categoria VARCHAR(100),
            ADD COLUMN IF NOT EXISTS llm_confianza FLOAT,
            ADD COLUMN IF NOT EXISTS llm_processed BOOLEAN DEFAULT FALSE,
            ADD COLUMN IF NOT EXISTS llm_processed_at TIMESTAMP;
        """)
        
        # Crear índice para procesamiento eficiente
        cur.execute("""
            CREATE INDEX IF NOT EXISTS idx_noticias_llm_processed 
            ON noticias(llm_processed) 
            WHERE llm_processed = FALSE;
        """)
        
        conn.commit()
    
    log.info("✓ Esquema verificado")


def fetch_unprocessed_news(conn, limit: int = 10) -> List[Dict]:
    """
    Obtiene noticias sin procesar, agrupadas por feed_id
    
    Estrategia:
    1. Obtiene una muestra de feeds con noticias pendientes
    2. Selecciona un feed aleatorio de esa muestra
    3. Obtiene hasta 'limit' noticias de ese feed específico
    
    Args:
        conn: Conexión a la base de datos
        limit: Número máximo de noticias a obtener
        
    Returns:
        Lista de diccionarios con noticias
    """
    import random
    
    with conn.cursor() as cur:
        # Paso 1: Identificar feeds candidatos
        # Tomamos una muestra de las noticias más recientes pendientes
        cur.execute("""
            SELECT feed_id
            FROM noticias
            WHERE llm_processed = FALSE
            ORDER BY fecha DESC
            LIMIT 100
        """)
        
        candidates = cur.fetchall()
        
        if not candidates:
            return []
            
        # Extraer IDs únicos de feeds y elegir uno al azar
        # Esto evita que un solo feed sature el worker (Round Robin pseudo-aleatorio)
        unique_feeds = list(set(r[0] for r in candidates if r[0] is not None))
        
        if not unique_feeds:
            return []
            
        target_feed_id = random.choice(unique_feeds)
        
        # Paso 2: Obtener lote del feed seleccionado
        cur.execute("""
            SELECT id, titulo, resumen
            FROM noticias
            WHERE llm_processed = FALSE
              AND feed_id = %s
              AND titulo IS NOT NULL
              AND resumen IS NOT NULL
            ORDER BY fecha DESC
            LIMIT %s
        """, (target_feed_id, limit))
        
        rows = cur.fetchall()
        
        log.info(f"Seleccionado feed_id {target_feed_id} para procesamiento ({len(rows)} items)")
    
    return [
        {
            'id': row[0],
            'titulo': row[1],
            'resumen': row[2]
        }
        for row in rows
    ]


def update_categorizations(conn, results: List[Dict]):
    """
    Actualiza las categorizaciones en la base de datos
    
    Args:
        conn: Conexión a la base de datos
        results: Lista de resultados de categorización
    """
    if not results:
        return
    
    with conn.cursor() as cur:
        # Preparar datos para update
        update_data = [
            (
                r['categoria'],
                r['confianza'],
                r['id']
            )
            for r in results
        ]
        
        # Actualizar en lote
        execute_values(cur, """
            UPDATE noticias AS n
            SET 
                llm_categoria = v.categoria,
                llm_confianza = v.confianza,
                llm_processed = TRUE,
                llm_processed_at = NOW()
            FROM (VALUES %s) AS v(categoria, confianza, id)
            WHERE n.id = v.id
        """, update_data)
        
        conn.commit()
    
    log.info(f"✓ Actualizadas {len(results)} noticias")


def main():
    """Main loop del worker"""
    log.info("=== Iniciando LLM Categorizer Worker ===")
    log.info(f"Batch size: {BATCH_SIZE}")
    log.info(f"Model path: {MODEL_PATH}")
    
    # Verificar que existe el modelo
    if not os.path.exists(os.path.join(MODEL_PATH, "config.json")):
        log.error(f"❌ Error: No se encuentra el modelo (config.json) en {MODEL_PATH}")
        log.error(f"Por favor descarga un modelo compatible (ej: Mistral-7B-Instruct-v0.2-GPTQ)")
        log.error(f"Ejecuta: ./scripts/download_llm_model.sh")
        # Dormir para no saturar logs si reinicia rápido
        time.sleep(60)
        sys.exit(1)
    
    # Inicializar esquema de base de datos
    try:
        with get_db_connection() as conn:
            initialize_schema(conn)
    except Exception as e:
        log.error(f"❌ Error inicializando esquema: {e}")
        sys.exit(1)
    
    # Cargar modelo
    try:
        categorizer = ExLlamaV2Categorizer(MODEL_PATH)
    except Exception as e:
        log.error(f"❌ Error cargando modelo: {e}")
        sys.exit(1)
    
    log.info("✓ Worker inicializado correctamente")
    log.info("Entrando en loop principal...")
    
    # Main loop
    while True:
        try:
            with get_db_connection() as conn:
                # Obtener noticias sin procesar
                news_items = fetch_unprocessed_news(conn, BATCH_SIZE)
                
                if not news_items:
                    log.debug(f"No hay noticias pendientes. Esperando {SLEEP_IDLE}s...")
                    time.sleep(SLEEP_IDLE)
                    continue
                
                log.info(f"Procesando {len(news_items)} noticias...")
                
                # Categorizar
                results = categorizer.categorize_news(news_items)
                
                # Actualizar base de datos
                update_categorizations(conn, results)
                
                # Estadísticas
                categories_count = {}
                for r in results:
                    cat = r['categoria']
                    categories_count[cat] = categories_count.get(cat, 0) + 1
                
                log.info(f"Distribución: {categories_count}")
                
                # Si procesamos el lote completo, continuar inmediatamente
                if len(news_items) < BATCH_SIZE:
                    time.sleep(SLEEP_IDLE)
                    
        except KeyboardInterrupt:
            log.info("Deteniendo worker...")
            break
        except Exception as e:
            log.exception(f"❌ Error en loop principal: {e}")
            time.sleep(SLEEP_IDLE)
    
    log.info("Worker finalizado")


if __name__ == "__main__":
    main()
