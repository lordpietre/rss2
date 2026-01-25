import os
from typing import List, Dict, Optional, Tuple, Any
from psycopg2 import extras
from utils.qdrant_search import semantic_search


def _extraer_tags_por_traduccion(cur, traduccion_ids: List[int]) -> Dict[int, List[tuple]]:
    """Obtiene tags agrupados por traducción."""
    tags_por_tr = {}

    if not traduccion_ids:
        return tags_por_tr

    cur.execute(
        """
        SELECT tn.traduccion_id, tg.valor, tg.tipo
        FROM tags_noticia tn
        JOIN tags tg ON tg.id = tn.tag_id
        WHERE tn.traduccion_id = ANY(%s);
        """,
        (traduccion_ids,),
    )
    rows = cur.fetchall()

    for tr_id, valor, tipo in rows:
        tags_por_tr.setdefault(tr_id, []).append((valor, tipo))

    return tags_por_tr


def buscar_noticias(
    conn,
    page: int,
    per_page: int,
    q: str = "",
    categoria_id: Optional[str] = None,
    continente_id: Optional[str] = None,
    pais_id: Optional[str] = None,
    fecha: Optional[str] = None,
    lang: str = "es",
    use_tr: bool = True,
    skip_count: bool = False,
) -> Tuple[List[Dict], int, int, Dict]:
    """
    Búsqueda avanzada de noticias con filtros:
    - fecha
    - país / continente
    - categoría
    - búsqueda fulltext + ILIKE
    - traducciones
    - paginación
    """
    offset = (page - 1) * per_page

    where = ["1=1"]
    params = []

    # Filtro por fecha exacta
    if fecha:
        where.append("n.fecha::date = %s")
        params.append(fecha)

    # Categoría
    if categoria_id:
        where.append("n.categoria_id = %s")
        params.append(int(categoria_id))

    # País o continente
    if pais_id:
        where.append("n.pais_id = %s")
        params.append(int(pais_id))
    elif continente_id:
        where.append("p.continente_id = %s")
        params.append(int(continente_id))

    # Búsqueda optimizada usando FTS (Full Text Search)
    if q:
        if use_tr:
            where.append(
                """
                (
                    n.search_vector_es @@ websearch_to_tsquery('spanish', %s)
                    OR t.search_vector_es @@ websearch_to_tsquery('spanish', %s)
                )
                """
            )
            params.extend([q, q])
        else:
            where.append("n.search_vector_es @@ websearch_to_tsquery('spanish', %s)")
            params.append(q)

    where_sql = " AND ".join(where)

    with conn.cursor(cursor_factory=extras.DictCursor) as cur:

        # =====================================================================
        # TOTAL DE RESULTADOS (OPTIMIZADO)
        # =====================================================================
        total_results = 0
        total_pages = 0
        
        if not skip_count:
            # Si no hay filtros de búsqueda de texto ni filtros complejos, usar estimación rápida
            if not q and not categoria_id and not pais_id and not continente_id and not fecha:
                 cur.execute("SELECT reltuples::bigint FROM pg_class WHERE relname = 'noticias'")
                 row = cur.fetchone()
                 total_results = row[0] if row else 0
            elif q and not (categoria_id or pais_id or continente_id or fecha):
                # Conteo optimizado para búsqueda simple (UNION de hits en noticias y traducciones)
                cur.execute(
                    """
                    SELECT COUNT(DISTINCT id) FROM (
                        SELECT id FROM noticias 
                        WHERE search_vector_es @@ websearch_to_tsquery('spanish', %s)
                        UNION ALL
                        SELECT noticia_id as id FROM traducciones 
                        WHERE search_vector_es @@ websearch_to_tsquery('spanish', %s) 
                          AND lang_to = %s AND status = 'done'
                    ) as all_hits
                    """,
                    (q, q, lang),
                )
                total_results = cur.fetchone()[0]
            else:
                # Conteo exacto si hay filtros combinados
                cur.execute(
                    f"""
                    SELECT COUNT(n.id)
                    FROM noticias n
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    LEFT JOIN traducciones t
                        ON t.noticia_id = n.id
                       AND t.lang_to = %s
                       AND t.status = 'done'
                    WHERE {where_sql}
                    """,
                    [lang] + params,
                )
                total_results = cur.fetchone()[0]
    
            total_pages = (total_results // per_page) + (1 if total_results % per_page else 0)

        # =====================================================================
        # LISTA DE NOTICIAS PAGINADAS
        # =====================================================================
        cur.execute(
            f"""
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

                -- traducciones
                t.id AS traduccion_id,
                t.titulo_trad AS titulo_traducido,
                t.resumen_trad AS resumen_traducido,
                CASE WHEN t.id IS NOT NULL THEN TRUE ELSE FALSE END AS tiene_traduccion,

                -- originales
                n.titulo AS titulo_original,
                n.resumen AS resumen_original

            FROM noticias n
            LEFT JOIN categorias c ON c.id = n.categoria_id
            LEFT JOIN paises p ON p.id = n.pais_id
            LEFT JOIN traducciones t
                ON t.noticia_id = n.id
               AND t.lang_to = %s
               AND t.status = 'done'
            WHERE {where_sql}
            ORDER BY n.fecha DESC NULLS LAST, n.id DESC
            LIMIT %s OFFSET %s
            """,
            [lang] + params + [per_page, offset],
        )
        noticias = cur.fetchall()

        # =====================================================================
        # TAGS POR TRADUCCIÓN
        # =====================================================================
        tr_ids = [n["traduccion_id"] for n in noticias if n["traduccion_id"]]
        tags_por_tr = _extraer_tags_por_traduccion(cur, tr_ids)

    return noticias, total_results, total_pages, tags_por_tr


# Embedding model loading moved to utils.qdrant_search

def buscar_noticias_semantica(
    conn,
    page: int,
    per_page: int,
    q: str,
    categoria_id: Optional[str] = None,
    continente_id: Optional[str] = None,
    pais_id: Optional[str] = None,
    fecha: Optional[Any] = None,
    lang: str = "es",
) -> Tuple[List[Dict], int, int, Dict]:
    """
    Búsqueda semántica optimizada usando Qdrant.
    Cae de vuelta a búsqueda tradicional si falla.
    """
    if not q.strip():
        return buscar_noticias(conn, page, per_page, "", categoria_id, continente_id, pais_id, fecha, lang)

    # Preparar filtros para Qdrant
    q_filters = {"lang": lang}
    if categoria_id:
        q_filters["categoria_id"] = int(categoria_id)
    if pais_id:
        q_filters["pais_id"] = int(pais_id)
    # Nota: No filtramos por fecha o continente en Qdrant por ahora para simplicidad,
    # ya que requeriría lógica más compleja de filtrado en Qdrant (rango o joins manuales).
    
    # Realizar búsqueda en Qdrant
    # Obtenemos más resultados de los necesarios para permitir re-filtrado o mejor ranking
    # Pero no demasiados para mantener la velocidad
    limit_qdrant = min(page * per_page * 2, 500)
    
    try:
        results_q = semantic_search(
            query=q,
            limit=limit_qdrant,
            score_threshold=0.35,
            filters=q_filters
        )
    except Exception as e:
        print(f"⚠️ Error en búsqueda Qdrant, usando fallback: {e}")
        return buscar_noticias(conn, page, per_page, q, categoria_id, continente_id, pais_id, fecha, lang)

    if not results_q:
        # Fallback a búsqueda tradicional si no hay resultados semánticos
        return buscar_noticias(conn, page, per_page, q, categoria_id, continente_id, pais_id, fecha, lang)

    # El total real en Qdrant para esta búsqueda es difícil de saber sin una query de conteo separada,
    # estimamos o usamos el tamaño de la lista retornada (limitada por nuestro umbral).
    total_results = len(results_q) 
    total_pages = (total_results // per_page) + (1 if total_results % per_page else 0)

    # Paginación sobre los resultados de Qdrant
    offset = (page - 1) * per_page
    paged_results_q = results_q[offset : offset + per_page]
    
    if not paged_results_q:
        return [], total_results, total_pages, {}

    # Enriquecer resultados con datos frescos de PostgreSQL
    news_ids = [r['news_id'] for r in paged_results_q]
    
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT
                n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url, n.fuente_nombre,
                c.nombre AS categoria, p.nombre AS pais,
                t.id AS traduccion_id, t.titulo_trad AS titulo_traducido, t.resumen_trad AS resumen_traducido,
                TRUE AS tiene_traduccion
            FROM noticias n
            LEFT JOIN categorias c ON c.id = n.categoria_id
            LEFT JOIN paises p ON p.id = n.pais_id
            LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = %s AND t.status = 'done'
            WHERE n.id = ANY(%s)
            """,
            (lang, news_ids),
        )
        db_rows = {row['id']: row for row in cur.fetchall()}
        
        # Mantener el orden de relevancia de Qdrant
        noticias_enriquecidas = []
        for r_q in paged_results_q:
            nid = r_q['news_id']
            if nid in db_rows:
                row = dict(db_rows[nid])
                row['score'] = r_q['score'] # Añadir score de relevancia
                noticias_enriquecidas.append(row)
        
        # Tags
        tr_ids = [n["traduccion_id"] for n in noticias_enriquecidas if n.get("traduccion_id")]
        tags_por_tr = _extraer_tags_por_traduccion(cur, tr_ids)

    return noticias_enriquecidas, total_results, total_pages, tags_por_tr
