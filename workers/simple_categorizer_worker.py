#!/usr/bin/env python3
"""
Simple Categorizer Worker - Categoriza noticias usando keywords
Procesa 10 noticias por feed de manera balanceada
"""

import os
import sys
import time
import logging
import random
import psycopg2
from psycopg2.extras import execute_values
from typing import List, Dict

logging.basicConfig(
    level=logging.INFO,
    format='[simple_categorizer] %(asctime)s %(levelname)s: %(message)s'
)
log = logging.getLogger(__name__)

DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", ""),
}

BATCH_SIZE = int(os.environ.get("CATEGORIZER_BATCH_SIZE", 10))
SLEEP_IDLE = int(os.environ.get("CATEGORIZER_SLEEP_IDLE", 5))

# Mapeo de keywords a categorías
CATEGORY_KEYWORDS = {
    "Ciencia": ["científico", "investigación", "estudio", "descubrimiento", "laboratorio", "experimento", "universidad", "研究", "science"],
    "Cultura": ["museo", "arte", "exposición", "artista", "cultura", "patrimonio", "文化", "culture"],
    "Deportes": ["fútbol", "deporte", "equipo", "partido", "jugador", "liga", "campeonato", "运动", "sport", "football"],
    "Economía": ["economía", "mercado", "empresa", "inversión", "bolsa", "financiero", "banco", "经济", "economy"],
    "Educación": ["educación", "escuela", "universidad", "estudiante", "profesor", "教育", "education"],
    "Entretenimiento": ["cine", "película", "actor", "música", "concierto", "娱乐", "entertainment", "film"],
    "Internacional": ["internacional", "país", "gobierno", "ministro", "外交", "international", "foreign"],
    "Medio Ambiente": ["clima", "ambiental", "contaminación", "ecología", "sostenible", "环境", "environment", "climate"],
    "Política": ["político", "gobierno", "presidente", "ministro", "parlamento", "elecciones", "政治", "politics"],
    "Salud": ["salud", "hospital", "médico", "enfermedad", "tratamiento", "健康", "health"],
    "Sociedad": ["social", "comunidad", "ciudadano", "población", "社会", "society"],
    "Tecnología": ["tecnología", "digital", "software", "internet", "app", "技术", "technology", "tech"],
}


def get_db_connection():
    return psycopg2.connect(**DB_CONFIG)


def categorize_by_keywords(titulo: str, resumen: str) -> tuple:
    """
    Returns: (category_name, confidence)
    """
    text = f"{titulo} {resumen}".lower()
    
    scores = {}
    for category, keywords in CATEGORY_KEYWORDS.items():
        score = sum(1 for kw in keywords if kw.lower() in text)
        if score > 0:
            scores[category] = score
    
    if not scores:
        return "Sociedad", 0.3  # Default
    
    best_category = max(scores, key=scores.get)
    max_score = scores[best_category]
    confidence = min(0.95, 0.5 + (max_score * 0.1))
    
    return best_category, confidence


def fetch_unprocessed_news(conn, limit: int = 10) -> List[Dict]:
    """Obtiene noticias que tienen traducción al español pero no han sido categorizadas"""
    with conn.cursor() as cur:
        # Obtener fuentes con noticias pendientes que tengan traducción 'done' en español
        cur.execute("""
            SELECT n.fuente_nombre
            FROM noticias n
            JOIN traducciones t ON n.id = t.noticia_id
            WHERE n.llm_processed = FALSE 
              AND t.lang_to = 'es'
              AND t.status = 'done'
            ORDER BY n.fecha DESC
            LIMIT 100
        """)
        
        candidates = cur.fetchall()
        if not candidates:
            return []
        
        unique_sources = list(set(r[0] for r in candidates if r[0] is not None))
        if not unique_sources:
            return []
        
        target_source = random.choice(unique_sources)
        
        # Obtener lote de la fuente seleccionada usando el texto traducido
        cur.execute("""
            SELECT n.id, t.titulo_trad, t.resumen_trad
            FROM noticias n
            JOIN traducciones t ON n.id = t.noticia_id
            WHERE n.llm_processed = FALSE
              AND n.fuente_nombre = %s
              AND t.lang_to = 'es'
              AND t.status = 'done'
              AND t.titulo_trad IS NOT NULL
            ORDER BY n.fecha DESC
            LIMIT %s
        """, (target_source, limit))
        
        rows = cur.fetchall()
        log.info(f"Seleccionada fuente '{target_source}' → {len(rows)} items (USANDO TRADUCCIÓN ES)")
    
    return [{'id': r[0], 'titulo': r[1], 'resumen': r[2]} for r in rows]


def update_categorizations(conn, results: List[Dict]):
    """Actualiza las categorizaciones"""
    if not results:
        return
    
    with conn.cursor() as cur:
        update_data = [
            (r['categoria'], r['confianza'], r['id'])
            for r in results
        ]
        
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
    
    log.info(f"✓ {len(results)} noticias categorizadas")


def main():
    log.info("=== Simple Categorizer Worker ===")
    log.info(f"Batch: {BATCH_SIZE} | Sleep: {SLEEP_IDLE}s")
    
    # Inicializar esquema
    try:
        log.info("Conectando a la base de datos para verificar esquema...")
        with get_db_connection() as conn:
            log.info("Conexión establecida. Verificando columnas...")
            with conn.cursor() as cur:
                cur.execute("""
                    ALTER TABLE noticias 
                    ADD COLUMN IF NOT EXISTS llm_categoria VARCHAR(100),
                    ADD COLUMN IF NOT EXISTS llm_confianza FLOAT,
                    ADD COLUMN IF NOT EXISTS llm_processed BOOLEAN DEFAULT FALSE,
                    ADD COLUMN IF NOT EXISTS llm_processed_at TIMESTAMP;
                """)
                cur.execute("""
                    CREATE INDEX IF NOT EXISTS idx_noticias_llm_processed 
                    ON noticias(llm_processed) 
                    WHERE llm_processed = FALSE;
                """)
                conn.commit()
        log.info("✓ Esquema verificado")
    except Exception as e:
        log.error(f"❌ Error inicializando: {e}")
        sys.exit(1)
    
    log.info("Entrando en loop principal...")
    
    while True:
        try:
            with get_db_connection() as conn:
                news_items = fetch_unprocessed_news(conn, BATCH_SIZE)
                
                if not news_items:
                    log.debug(f"Sin noticias pendientes. Sleep {SLEEP_IDLE}s...")
                    time.sleep(SLEEP_IDLE)
                    continue
                
                log.info(f"Procesando {len(news_items)} noticias...")
                
                results = []
                for item in news_items:
                    categoria, confianza = categorize_by_keywords(
                        item['titulo'], 
                        item['resumen']
                    )
                    results.append({
                        'id': item['id'],
                        'categoria': categoria,
                        'confianza': confianza
                    })
                
                update_categorizations(conn, results)
                
                # Estadísticas
                stats = {}
                for r in results:
                    cat = r['categoria']
                    stats[cat] = stats.get(cat, 0) + 1
                
                log.info(f"Distribución: {stats}")
                
                if len(news_items) < BATCH_SIZE:
                    time.sleep(SLEEP_IDLE)
                    
        except KeyboardInterrupt:
            log.info("Deteniendo worker...")
            break
        except Exception as e:
            log.exception(f"❌ Error: {e}")
            time.sleep(SLEEP_IDLE)
    
    log.info("Worker finalizado")


if __name__ == "__main__":
    main()
