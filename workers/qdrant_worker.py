"""
Worker de Qdrant
Vectoriza noticias traducidas y las sube a Qdrant para búsquedas semánticas.
"""

import os
import sys
import time
import uuid
from datetime import datetime
from typing import List, Dict, Any

# Añadir el directorio raíz al path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from db import get_read_conn, get_write_conn

try:
    from qdrant_client import QdrantClient
    from qdrant_client.models import Distance, VectorParams, PointStruct, Filter, FieldCondition, MatchValue
except ImportError:
    print("❌ Error: qdrant-client no instalado. Ejecuta: pip install qdrant-client")
    sys.exit(1)

try:
    from sentence_transformers import SentenceTransformer
except ImportError:
    print("❌ Error: sentence-transformers no instalado")
    sys.exit(1)

# Configuración
QDRANT_HOST = os.environ.get("QDRANT_HOST", "localhost")
QDRANT_PORT = int(os.environ.get("QDRANT_PORT", "6333"))
QDRANT_COLLECTION = os.environ.get("QDRANT_COLLECTION_NAME", "news_vectors")

EMB_MODEL = os.environ.get("EMB_MODEL", "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2")
EMB_DEVICE = os.environ.get("EMB_DEVICE", "cuda")
BATCH_SIZE = int(os.environ.get("QDRANT_BATCH_SIZE", "100"))
SLEEP_IDLE = int(os.environ.get("QDRANT_SLEEP_IDLE", "30"))

# Cliente Qdrant global
qdrant_client = None
embedding_model = None


def init_qdrant_client():
    """
    Inicializa el cliente de Qdrant y crea la colección si no existe.
    """
    global qdrant_client
    
    print(f"🔌 Conectando a Qdrant en {QDRANT_HOST}:{QDRANT_PORT}...")
    qdrant_client = QdrantClient(host=QDRANT_HOST, port=QDRANT_PORT)
    
    # Verificar si la colección existe
    collections = qdrant_client.get_collections().collections
    collection_names = [c.name for c in collections]
    
    if QDRANT_COLLECTION not in collection_names:
        print(f"📦 Creando colección '{QDRANT_COLLECTION}'...")
        
        # Obtener dimensión del modelo de embeddings
        # paraphrase-multilingual-MiniLM-L12-v2 = 384 dimensiones
        vector_size = 384
        
        qdrant_client.create_collection(
            collection_name=QDRANT_COLLECTION,
            vectors_config=VectorParams(
                size=vector_size,
                distance=Distance.COSINE
            )
        )
        print(f"✅ Colección '{QDRANT_COLLECTION}' creada (dimensión: {vector_size})")
    else:
        print(f"✅ Colección '{QDRANT_COLLECTION}' ya existe")
    
    # Obtener info de la colección
    collection_info = qdrant_client.get_collection(QDRANT_COLLECTION)
    print(f"📊 Puntos en colección: {collection_info.points_count}")


def init_embedding_model():
    """
    Inicializa el modelo de embeddings.
    """
    global embedding_model
    
    print(f"🤖 Cargando modelo de embeddings: {EMB_MODEL}")
    print(f"🖥️  Dispositivo: {EMB_DEVICE}")
    
    embedding_model = SentenceTransformer(EMB_MODEL, device=EMB_DEVICE)
    
    print(f"✅ Modelo cargado correctamente")


def get_pending_news(limit: int = BATCH_SIZE) -> List[Dict[str, Any]]:
    """
    Obtiene noticias traducidas pendientes de vectorizar.
    
    Args:
        limit: Número máximo de noticias a obtener
        
    Returns:
        Lista de noticias
    """
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT 
                    t.id as traduccion_id,
                    t.noticia_id,
                    TRIM(t.lang_to) as lang,
                    t.titulo_trad as titulo,
                    t.resumen_trad as resumen,
                    n.url,
                    n.fecha,
                    n.fuente_nombre,
                    n.categoria_id,
                    n.pais_id
                FROM traducciones t
                INNER JOIN noticias n ON t.noticia_id = n.id
                WHERE t.vectorized = FALSE
                AND t.status = 'done'
                ORDER BY t.created_at ASC
                LIMIT %s
            """, (limit,))
            
            columns = [desc[0] for desc in cur.description]
            results = []
            for row in cur.fetchall():
                results.append(dict(zip(columns, row)))
            
            return results


def generate_embeddings(texts: List[str]) -> List[List[float]]:
    """
    Genera embeddings para una lista de textos.
    
    Args:
        texts: Lista de textos
        
    Returns:
        Lista de vectores de embeddings
    """
    embeddings = embedding_model.encode(
        texts,
        batch_size=32,
        show_progress_bar=False,
        convert_to_numpy=True
    )
    return embeddings.tolist()


def upload_to_qdrant(news_batch: List[Dict[str, Any]]):
    """
    Sube un lote de noticias a Qdrant.
    
    Args:
        news_batch: Lista de noticias
    """
    if not news_batch:
        return
    
    # Preparar textos para embeddings (título + resumen)
    texts = [
        f"{news['titulo']} {news['resumen']}"
        for news in news_batch
    ]
    
    print(f"  🧮 Generando embeddings para {len(texts)} noticias...")
    embeddings = generate_embeddings(texts)
    
    # Preparar puntos para Qdrant
    points = []
    for news, embedding in zip(news_batch, embeddings):
        point_id = str(uuid.uuid4())
        
        # Preparar payload (metadata)
        payload = {
            "news_id": news['noticia_id'],
            "traduccion_id": news['traduccion_id'],
            "titulo": news['titulo'],
            "resumen": news['resumen'],
            "url": news['url'],
            "fecha": news['fecha'].isoformat() if news['fecha'] else None,
            "fuente_nombre": news['fuente_nombre'],
            "categoria_id": news['categoria_id'],
            "pais_id": news['pais_id'],
            "lang": news['lang']
        }
        
        point = PointStruct(
            id=point_id,
            vector=embedding,
            payload=payload
        )
        points.append(point)
        
        # Guardar point_id para actualizar DB
        news['qdrant_point_id'] = point_id
    
    # Subir a Qdrant
    print(f"  ⬆️  Subiendo {len(points)} puntos a Qdrant...")
    qdrant_client.upsert(
        collection_name=QDRANT_COLLECTION,
        points=points
    )
    
    # Actualizar base de datos
    print(f"  💾 Actualizando estado en PostgreSQL...")
    with get_write_conn() as conn:
        with conn.cursor() as cur:
            for news in news_batch:
                cur.execute("""
                    UPDATE traducciones
                    SET 
                        vectorized = TRUE,
                        vectorization_date = NOW(),
                        qdrant_point_id = %s
                    WHERE id = %s
                """, (news['qdrant_point_id'], news['traduccion_id']))
        conn.commit()
    
    print(f"  ✅ Lote subido correctamente")


def process_batch():
    """
    Procesa un lote de noticias traducidas.
    
    Returns:
        Número de noticias procesadas
    """
    news_batch = get_pending_news()
    
    if not news_batch:
        return 0
    
    print(f"\n📋 Procesando {len(news_batch)} noticias traducidas...")
    
    try:
        upload_to_qdrant(news_batch)
        return len(news_batch)
    except Exception as e:
        print(f"❌ Error procesando lote: {e}")
        return 0


def get_stats():
    """
    Obtiene estadísticas del sistema.
    """
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT 
                    COUNT(*) as total,
                    COUNT(*) FILTER (WHERE vectorized = TRUE) as vectorizadas,
                    COUNT(*) FILTER (WHERE vectorized = FALSE AND status = 'done') as pendientes
                FROM traducciones
                WHERE lang_to = 'es' 
            """)
            row = cur.fetchone()
            return {
                'total': row[0],
                'vectorizadas': row[1],
                'pendientes': row[2]
            }


def main():
    """
    Loop principal del worker.
    """
    print("=" * 80)
    print("🚀 Qdrant Vectorization Worker (Direct Translation)")
    print("=" * 80)
    print(f"Qdrant: {QDRANT_HOST}:{QDRANT_PORT}")
    print(f"Colección: {QDRANT_COLLECTION}")
    print(f"Modelo: {EMB_MODEL}")
    print(f"Dispositivo: {EMB_DEVICE}")
    print(f"Tamaño de lote: {BATCH_SIZE}")
    print("=" * 80)
    
    # Inicializar Qdrant
    try:
        init_qdrant_client()
    except Exception as e:
        print(f"❌ Error inicializando Qdrant: {e}")
        print("⚠️  Asegúrate de que Qdrant esté corriendo")
        return
    
    # Inicializar modelo de embeddings
    try:
        init_embedding_model()
    except Exception as e:
        print(f"❌ Error cargando modelo de embeddings: {e}")
        return
    
    print("\n🔄 Iniciando loop de procesamiento...\n")
    
    total_processed = 0
    
    while True:
        try:
            processed = process_batch()
            total_processed += processed
            
            if processed > 0:
                print(f"\n✅ Lote completado: {processed} noticias vectorizadas")
                print(f"📊 Total procesado en esta sesión: {total_processed}")
                
                # Mostrar estadísticas
                stats = get_stats()
                print(f"📈 Estadísticas globales:")
                print(f"   Total traducciones: {stats['total']}")
                print(f"   Vectorizadas: {stats['vectorizadas']}")
                print(f"   Pendientes: {stats['pendientes']}")
            else:
                print(f"💤 No hay noticias pendientes. Esperando {SLEEP_IDLE}s...")
                time.sleep(SLEEP_IDLE)
                
        except KeyboardInterrupt:
            print("\n\n⏹️  Worker detenido por el usuario")
            break
        except Exception as e:
            print(f"\n❌ Error en loop principal: {e}")
            print(f"⏳ Esperando {SLEEP_IDLE}s antes de reintentar...")
            time.sleep(SLEEP_IDLE)


if __name__ == "__main__":
    main()
