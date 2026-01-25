"""
Utilidad de búsqueda semántica con Qdrant.
Proporciona búsquedas vectoriales rápidas para noticias.
"""
import os
import time
from typing import List, Dict, Any, Optional
from qdrant_client import QdrantClient
# from sentence_transformers import SentenceTransformer (Moved to function)

# Configuración
QDRANT_HOST = os.environ.get("QDRANT_HOST", "localhost")
QDRANT_PORT = int(os.environ.get("QDRANT_PORT", "6333"))
QDRANT_COLLECTION = os.environ.get("QDRANT_COLLECTION_NAME", "news_vectors")
EMB_MODEL = os.environ.get("EMB_MODEL", "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2")
EMB_DEVICE = os.environ.get("EMB_DEVICE", "cpu")  # Default to CPU, but check env

# Singleton para clientes globales
_qdrant_client: Optional[QdrantClient] = None
_embedding_model: Optional[Any] = None


def get_qdrant_client() -> QdrantClient:
    """
    Obtiene el cliente de Qdrant (singleton).
    Incluye verificación de salud y manejo de errores.
    """
    global _qdrant_client
    if _qdrant_client is None:
        try:
            print(f"🔌 Conectando a Qdrant en {QDRANT_HOST}:{QDRANT_PORT}")
            _qdrant_client = QdrantClient(host=QDRANT_HOST, port=QDRANT_PORT, timeout=5)
            
            # Health check
            collections = _qdrant_client.get_collections()
            print(f"✅ Qdrant conectado. Colecciones: {[c.name for c in collections.collections]}")
        except Exception as e:
            print(f"❌ Error conectando a Qdrant: {e}")
            _qdrant_client = None
            raise
    return _qdrant_client


def get_embedding_model() -> Any:
    """
    Obtiene el modelo de embeddings (singleton).
    """
    global _embedding_model
    if _embedding_model is None:
        from sentence_transformers import SentenceTransformer
        import torch
        
        device = EMB_DEVICE
        if device == "cuda" and not torch.cuda.is_available():
            print("⚠️ CUDA solicitado pero no disponible, usando CPU")
            device = "cpu"
            
        print(f"🤖 Cargando modelo de embeddings: {EMB_MODEL} en {device}")
        _embedding_model = SentenceTransformer(EMB_MODEL, device=device)
    return _embedding_model


def semantic_search(
    query: str,
    limit: int = 20,
    score_threshold: float = 0.5,
    filters: Optional[Dict[str, Any]] = None
) -> List[Dict[str, Any]]:
    """
    Realiza una búsqueda semántica en Qdrant.
    """
    start_total = time.time()
    try:
        # Generar embedding de la consulta
        t0 = time.time()
        model = get_embedding_model()
        query_vector = model.encode(query, convert_to_numpy=True).tolist()
        t1 = time.time()
        print(f"⏱️ [Timing] Generar embedding de query: {t1 - t0:.4f}s")
        
        # Realizar búsqueda
        try:
            client = get_qdrant_client()
        except Exception as conn_error:
            print(f"⚠️ No se pudo conectar a Qdrant: {conn_error}")
            return []  # Retornar lista vacía para activar fallback
        
        search_params = {
            "collection_name": QDRANT_COLLECTION,
            "query_vector": query_vector,
            "limit": limit,
            "score_threshold": score_threshold
        }
        
        # Agregar filtros si existen
        if filters:
            from qdrant_client.models import Filter, FieldCondition, MatchValue
            
            conditions = []
            for key, value in filters.items():
                if value is not None:
                    if key == "lang" and isinstance(value, str) and len(value) < 5:
                        # Character(5) in Postgres pads with spaces
                        value = value.ljust(5)
                    
                    conditions.append(
                        FieldCondition(key=key, match=MatchValue(value=value))
                    )
            
            if conditions:
                search_params["query_filter"] = Filter(must=conditions)
        
        t2 = time.time()
        results = client.search(**search_params)
        t3 = time.time()
        print(f"⏱️ [Timing] Búsqueda en Qdrant: {t3 - t2:.4f}s")
        print(f"⏱️ [Timing] Total semantic_search: {t3 - start_total:.4f}s")
        print(f"✅ Qdrant retornó {len(results)} resultados")
        
        # Formatear resultados
        formatted_results = []
        for hit in results:
            formatted_results.append({
                "score": hit.score,
                "news_id": hit.payload.get("news_id"),
                "traduccion_id": hit.payload.get("traduccion_id"),
                "titulo": hit.payload.get("titulo", ""),
                "resumen": hit.payload.get("resumen", ""),
                "url": hit.payload.get("url", ""),
                "fecha": hit.payload.get("fecha"),
                "fuente_nombre": hit.payload.get("fuente_nombre", ""),
                "categoria_id": hit.payload.get("categoria_id"),
                "pais_id": hit.payload.get("pais_id"),
                "lang": hit.payload.get("lang", "es")
            })
        
        return formatted_results
        
    except Exception as e:
        print(f"❌ Error en búsqueda semántica: {e}")
        import traceback
        traceback.print_exc()
        return []


def hybrid_search(
    query: str,
    limit: int = 20,
    semantic_weight: float = 0.7,
    filters: Optional[Dict[str, Any]] = None
) -> List[Dict[str, Any]]:
    """
    Búsqueda híbrida: combina búsqueda semántica (Qdrant) con búsqueda tradicional.
    
    Args:
        query: Texto de búsqueda
        limit: Número máximo de resultados
        semantic_weight: Peso de la búsqueda semántica (0-1)
        filters: Filtros adicionales
        
    Returns:
        Lista de resultados combinados
    """
    # Por ahora, solo usar búsqueda semántica
    # TODO: Implementar combinación con búsqueda PostgreSQL en futuro si es necesario
    return semantic_search(query, limit=limit, filters=filters)


def search_by_keywords(
    keywords: List[str],
    limit: int = 100,
    score_threshold: float = 0.4
) -> List[Dict[str, Any]]:
    """
    Búsqueda por múltiples palabras clave.
    Útil para el monitor de conflictos.
    
    Args:
        keywords: Lista de palabras clave
        limit: Número máximo de resultados por keyword
        score_threshold: Umbral mínimo de similitud
        
    Returns:
        Lista de resultados únicos
    """
    all_results = {}
    
    for keyword in keywords:
        if not keyword.strip():
            continue
            
        results = semantic_search(
            query=keyword,
            limit=limit,
            score_threshold=score_threshold
        )
        
        # Agregar a resultados, manteniendo el mejor score
        for result in results:
            news_id = result['news_id']
            if news_id not in all_results or result['score'] > all_results[news_id]['score']:
                all_results[news_id] = result
    
    # Ordenar por score descendente
    sorted_results = sorted(
        all_results.values(),
        key=lambda x: x['score'],
        reverse=True
    )
    
    return sorted_results[:limit]
