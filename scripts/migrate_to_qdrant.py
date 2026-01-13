#!/usr/bin/env python3
"""
Script de migración para vectorizar noticias existentes en Qdrant.

Uso:
    # Ver estadísticas
    python scripts/migrate_to_qdrant.py --stats
    
    # Vectorizar noticias (proceso completo)
    python scripts/migrate_to_qdrant.py --vectorize --batch-size 200
    
    # Limpiar y empezar de nuevo
    python scripts/migrate_to_qdrant.py --reset
"""

import os
import sys
import argparse
import time
from datetime import datetime

# Añadir el directorio raíz al path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from db import get_read_conn, get_write_conn


def get_statistics():
    """
    Muestra estadísticas del sistema.
    """
    print("\n" + "=" * 80)
    print("📊 ESTADÍSTICAS DEL SISTEMA")
    print("=" * 80)
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            # Traducciones totales
            cur.execute("""
                SELECT 
                    COUNT(*) as total,
                    COUNT(*) FILTER (WHERE lang_to = 'es') as es,
                    COUNT(*) FILTER (WHERE status = 'done') as completadas
                FROM traducciones
            """)
            row = cur.fetchone()
            print(f"\n📰 TRADUCCIONES:")
            print(f"   Total: {row[0]:,}")
            print(f"   En español: {row[1]:,}")
            print(f"   Completadas: {row[2]:,}")
            
            # Estado vectorización
            cur.execute("""
                SELECT 
                    COUNT(*) as total,
                    COUNT(*) FILTER (WHERE vectorized = TRUE) as vectorizadas,
                    COUNT(*) FILTER (WHERE vectorized = FALSE AND status = 'done') as pendientes
                FROM traducciones
                WHERE lang_to = 'es'
            """)
            row = cur.fetchone()
            print(f"\n🔧 VECTORIZACIÓN:")
            print(f"   Total (ES): {row[0]:,}")
            print(f"   Vectorizadas: {row[1]:,}")
            print(f"   Pendientes: {row[2]:,}")
            
            # Info de Qdrant (si existe)
            try:
                from qdrant_client import QdrantClient
                qdrant_host = os.environ.get("QDRANT_HOST", "localhost")
                qdrant_port = int(os.environ.get("QDRANT_PORT", "6333"))
                collection_name = os.environ.get("QDRANT_COLLECTION_NAME", "news_vectors")
                
                client = QdrantClient(host=qdrant_host, port=qdrant_port)
                collection_info = client.get_collection(collection_name)
                
                print(f"\n🔍 QDRANT:")
                print(f"   Colección: {collection_name}")
                print(f"   Puntos: {collection_info.points_count:,}")
                print(f"   Vectores: {collection_info.vectors_count:,}")
            except Exception as e:
                print(f"\n⚠️  No se pudo conectar a Qdrant: {e}")
    
    print("\n" + "=" * 80 + "\n")


def vectorize_all(batch_size: int = 200):
    """
    Vectoriza todas las noticias traducidas pendientes.
    """
    print("\n" + "=" * 80)
    print("🔍 INICIANDO VECTORIZACIÓN MASIVA")
    print("=" * 80)
    print(f"Tamaño de lote: {batch_size}")
    print("=" * 80 + "\n")
    
    # Importar el worker de Qdrant
    from workers.qdrant_worker import (
        init_qdrant_client,
        init_embedding_model,
        get_pending_news,
        upload_to_qdrant
    )
    
    # Inicializar
    print("🔌 Inicializando Qdrant...")
    init_qdrant_client()
    
    print("🤖 Cargando modelo de embeddings...")
    init_embedding_model()
    
    total_processed = 0
    start_time = time.time()
    
    while True:
        # Obtener lote pendiente
        news_batch = get_pending_news(limit=batch_size)
        
        if not news_batch:
            print("\n✅ No hay más noticias pendientes de vectorizar")
            break
        
        print(f"\n📋 Procesando lote de {len(news_batch)} noticias...")
        
        try:
            upload_to_qdrant(news_batch)
            total_processed += len(news_batch)
            
            elapsed = time.time() - start_time
            rate = total_processed / elapsed if elapsed > 0 else 0
            
            print(f"\n📊 Progreso: {total_processed:,} vectorizadas")
            print(f"⏱️  Velocidad: {rate:.2f} noticias/segundo")
            print(f"⏳ Tiempo transcurrido: {elapsed/60:.1f} minutos")
            
        except Exception as e:
            print(f"❌ Error procesando lote: {e}")
            break
    
    elapsed = time.time() - start_time
    print("\n" + "=" * 80)
    print("✅ VECTORIZACIÓN COMPLETADA")
    print("=" * 80)
    print(f"Total vectorizadas: {total_processed:,}")
    print(f"Tiempo total: {elapsed/60:.1f} minutos")
    print(f"Velocidad promedio: {total_processed/elapsed:.2f} noticias/segundo")
    print("=" * 80 + "\n")


def reset_all():
    """
    Resetea el estado de vectorización y limpia Qdrant.
    """
    print("\n" + "=" * 80)
    print("⚠️  RESET COMPLETO DEL SISTEMA DE VECTORES")
    print("=" * 80)
    
    response = input("\n¿Estás seguro? Esto eliminará TODOS los vectores y reiniciará el estado (s/N): ")
    
    if response.lower() != 's':
        print("❌ Operación cancelada")
        return
    
    print("\n🗑️  Reseteando base de datos...")
    
    with get_write_conn() as conn:
        with conn.cursor() as cur:
            # Resetear flag de vectorización
            cur.execute("""
                UPDATE traducciones 
                SET vectorized = FALSE, 
                    qdrant_point_id = NULL,
                    vectorization_date = NULL
            """)
        conn.commit()
    
    print("✅ Flags de vectorización reseteados en PostgreSQL")
    
    # Limpiar Qdrant
    try:
        from qdrant_client import QdrantClient
        qdrant_host = os.environ.get("QDRANT_HOST", "localhost")
        qdrant_port = int(os.environ.get("QDRANT_PORT", "6333"))
        collection_name = os.environ.get("QDRANT_COLLECTION_NAME", "news_vectors")
        
        client = QdrantClient(host=qdrant_host, port=qdrant_port)
        
        # Eliminar colección
        client.delete_collection(collection_name)
        print(f"✅ Colección '{collection_name}' eliminada de Qdrant")
        
        # Recrear colección
        from qdrant_client.models import Distance, VectorParams
        client.create_collection(
            collection_name=collection_name,
            vectors_config=VectorParams(size=384, distance=Distance.COSINE)
        )
        print(f"✅ Colección '{collection_name}' recreada")
        
    except Exception as e:
        print(f"⚠️  Error limpiando Qdrant: {e}")
    
    print("\n✅ Reset completado\n")


def main():
    parser = argparse.ArgumentParser(
        description="Script de migración para Qdrant (Directo)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    
    parser.add_argument("--stats", action="store_true", help="Mostrar estadísticas")
    parser.add_argument("--vectorize", action="store_true", help="Vectorizar noticias traducidas")
    parser.add_argument("--reset", action="store_true", help="Limpiar Qdrant y reiniciar estado")
    parser.add_argument("--batch-size", type=int, default=200, help="Tamaño de lote (default: 200)")
    
    args = parser.parse_args()
    
    # Si no se especifica ninguna opción, mostrar estadísticas
    if not any([args.stats, args.vectorize, args.reset]):
        args.stats = True
    
    try:
        if args.stats:
            get_statistics()
        
        if args.reset:
            reset_all()
        
        if args.vectorize:
            vectorize_all(batch_size=args.batch_size)
            
    except KeyboardInterrupt:
        print("\n\n⏹️  Proceso interrumpido por el usuario")
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
