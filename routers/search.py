"""
Search API router - Real-time search with semantic search (Qdrant) and autocomplete.
"""
from flask import Blueprint, request, jsonify
from psycopg2 import extras
from db import get_read_conn, get_write_conn
from utils.auth import get_current_user
from utils.qdrant_search import semantic_search

search_bp = Blueprint("search", __name__, url_prefix="/api/search")


@search_bp.route("/")
def search():
    """Search noticias using semantic search (Qdrant) with PostgreSQL fallback."""
    q = (request.args.get("q") or "").strip()
    limit = min(int(request.args.get("limit", 10)), 50)
    page = max(int(request.args.get("page", 1)), 1)  # Página actual (1-indexed)
    offset = (page - 1) * limit  # Calcular offset
    lang = (request.args.get("lang") or "es").lower()[:5]
    use_semantic = request.args.get("semantic", "true").lower() == "true"
    
    if not q or len(q) < 2:
        return jsonify({
            "results": [], 
            "total": 0,
            "page": page,
            "limit": limit,
            "total_pages": 0
        })
    
    results = []
    total = 0
    
    # Intentar búsqueda semántica primero (más rápida y mejor)
    if use_semantic:
        try:
            # Para paginación, obtenemos más resultados de Qdrant
            # Qdrant es muy rápido, así que podemos obtener bastantes resultados
            max_qdrant_results = min(offset + limit * 3, 200)  # Obtener hasta 3 páginas adelante
            
            semantic_results = semantic_search(
                query=q,
                limit=max_qdrant_results,
                score_threshold=0.3,  # Umbral más bajo para capturar más resultados
                filters={"lang": lang}
            )
            
            if semantic_results:
                # Calcular total encontrado (hasta el límite de fetching)
                total = len(semantic_results)
                
                # Obtener solo los resultados de la página actual
                page_results = semantic_results[offset : offset + limit]
                
                if page_results:
                    # Enriquecer con datos adicionales de PostgreSQL solo para esta página
                    news_ids = [r['news_id'] for r in page_results]
                    
                    with get_read_conn() as conn:
                        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                            # Obtener datos adicionales (categoría, país)
                            cur.execute("""
                                SELECT 
                                    n.id,
                                    n.titulo,
                                    n.resumen,
                                    n.url,
                                    n.fecha,
                                    n.imagen_url,
                                    n.fuente_nombre,
                                    c.nombre AS categoria,
                                    p.nombre AS pais,
                                    t.titulo_trad,
                                    t.resumen_trad,
                                    t.id AS traduccion_id
                                FROM noticias n
                                LEFT JOIN categorias c ON c.id = n.categoria_id
                                LEFT JOIN paises p ON p.id = n.pais_id
                                LEFT JOIN traducciones t ON t.noticia_id = n.id 
                                    AND t.lang_to = %s AND t.status = 'done'
                                WHERE n.id = ANY(%s)
                            """, (lang, news_ids))
                            
                            db_rows = {row['id']: row for row in cur.fetchall()}
                    
                    # Combinar resultados semánticos con datos de PostgreSQL
                    for sem_result in page_results:
                        news_id = sem_result['news_id']
                        db_row = db_rows.get(news_id)
                        
                        if db_row:
                            results.append({
                                "id": db_row["id"],
                                "titulo": db_row["titulo_trad"] or db_row["titulo"],
                                "resumen": (db_row["resumen_trad"] or db_row["resumen"] or "")[:150],
                                "url": db_row["url"],
                                "fecha": db_row["fecha"].isoformat() if db_row["fecha"] else None,
                                "imagen_url": db_row["imagen_url"],
                                "fuente": db_row["fuente_nombre"],
                                "categoria": db_row["categoria"],
                                "pais": db_row["pais"],
                                "traduccion_id": db_row["traduccion_id"],
                                "semantic_score": sem_result['score'],
                                "fecha_raw": db_row["fecha"]  # Para ordenación
                            })
                    
                    # Ordenar por fecha cronológicamente (más reciente primero)
                    results.sort(key=lambda x: x.get("fecha_raw") or "", reverse=True)
                    
                    # Eliminar el campo temporal usado para ordenación
                    for r in results:
                        r.pop("fecha_raw", None)
                
        except Exception as e:
            print(f"⚠️ Error en búsqueda semántica, usando fallback: {e}")
            import traceback
            traceback.print_exc()
            # Continuar con búsqueda tradicional
    
    # Fallback a búsqueda tradicional si no hay resultados semánticos y no hubo error fatal
    if not results and total == 0:
        with get_read_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                print(f"⚠️ Usando fallback PostgreSQL para búsqueda: '{q}'")
                
                # Búsqueda tradicional optimizada usando Full Text Search
                # Nota: Esta query es más lenta que Qdrant pero necesaria como fallback
                cur.execute("""
                    WITH ranked_news AS (
                        -- Búsqueda en noticias originales
                        SELECT 
                            n.id,
                            ts_rank(n.search_vector_es, websearch_to_tsquery('spanish', %s)) as rank
                        FROM noticias n
                        WHERE n.search_vector_es @@ websearch_to_tsquery('spanish', %s)
                        
                        UNION ALL
                        
                        -- Búsqueda en traducciones
                        SELECT 
                            t.noticia_id as id,
                            ts_rank(t.search_vector_es, websearch_to_tsquery('spanish', %s)) as rank
                        FROM traducciones t
                        WHERE t.search_vector_es @@ websearch_to_tsquery('spanish', %s)
                          AND t.lang_to = 'es' 
                          AND t.status = 'done'
                    ),
                    best_ranks AS (
                        SELECT id, MAX(rank) as max_rank
                        FROM ranked_news
                        GROUP BY id
                    )
                    SELECT 
                        n.id,
                        n.titulo,
                        n.resumen,
                        n.url,
                        n.fecha,
                        n.imagen_url,
                        n.fuente_nombre,
                        c.nombre AS categoria,
                        p.nombre AS pais,
                        t.titulo_trad,
                        t.resumen_trad,
                        t.id AS traduccion_id,
                        br.max_rank AS rank
                    FROM best_ranks br
                    JOIN noticias n ON n.id = br.id
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    LEFT JOIN traducciones t ON t.noticia_id = n.id 
                        AND t.lang_to = %s AND t.status = 'done'
                    ORDER BY n.fecha DESC, br.max_rank DESC
                    LIMIT %s OFFSET %s
                """, (q, q, q, q, lang, limit, offset))
                
                rows = cur.fetchall()
                print(f"✅ PostgreSQL retornó {len(rows)} resultados")
                
                # Count total - Query simplificada
                cur.execute("""
                    SELECT COUNT(DISTINCT id) FROM (
                        SELECT id FROM noticias 
                        WHERE search_vector_es @@ websearch_to_tsquery('spanish', %s)
                        UNION
                        SELECT noticia_id as id FROM traducciones 
                        WHERE search_vector_es @@ websearch_to_tsquery('spanish', %s) 
                          AND lang_to = 'es' AND status = 'done'
                    ) as all_hits
                """, (q, q))
                total_row = cur.fetchone()
                total = total_row[0] if total_row else 0
        
        for r in rows:
            results.append({
                "id": r["id"],
                "titulo": r["titulo_trad"] or r["titulo"],
                "resumen": (r["resumen_trad"] or r["resumen"] or "")[:150],
                "url": r["url"],
                "fecha": r["fecha"].isoformat() if r["fecha"] else None,
                "imagen_url": r["imagen_url"],
                "fuente": r["fuente_nombre"],
                "categoria": r["categoria"],
                "pais": r["pais"],
                "traduccion_id": r["traduccion_id"],
            })
    
    # Save search history for authenticated users
    user = get_current_user()
    if user and q and page == 1: # Solo guardar en página 1
        try:
            with get_write_conn() as conn:
                with conn.cursor() as cur:
                    cur.execute("""
                        INSERT INTO search_history (user_id, query, results_count)
                        VALUES (%s, %s, %s)
                    """, (user['id'], q, total))
                conn.commit()
        except Exception as e:
            print(f"ERROR SAVING SEARCH HISTORY: {e}")
            pass
    
    total_pages = (total + limit - 1) // limit if limit > 0 else 0
    
    return jsonify({
        "results": results, 
        "total": total, 
        "query": q,
        "page": page,
        "limit": limit,
        "total_pages": total_pages
    })


@search_bp.route("/suggestions")
def suggestions():
    """Get search suggestions based on recent/popular searches and tags."""
    q = (request.args.get("q") or "").strip()
    limit = min(int(request.args.get("limit", 5)), 10)
    
    if not q or len(q) < 2:
        return jsonify({"suggestions": []})
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            # Get matching tags as suggestions
            cur.execute("""
                SELECT DISTINCT valor
                FROM tags
                WHERE valor ILIKE %s
                ORDER BY valor
                LIMIT %s;
            """, (f"%{q}%", limit))
            
            suggestions = [row[0] for row in cur.fetchall()]
    
    return jsonify({"suggestions": suggestions, "query": q})
