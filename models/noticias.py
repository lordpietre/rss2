from psycopg2 import extras
from typing import List, Dict, Optional, Tuple, Any
import os
# from sentence_transformers import SentenceTransformer (Moved to functions to avoid heavy start-up)


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
            else:
                # Conteo exacto si hay filtros (necesario para paginación filtrada)
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


# Cache del modelo para no cargarlo en cada petición
_model_cache = {}

def _get_emb_model():
    from sentence_transformers import SentenceTransformer
    model_name = os.environ.get("EMB_MODEL", "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2")
    if model_name not in _model_cache:
        device = "cuda" if torch.cuda.is_available() else "cpu"
        _model_cache[model_name] = SentenceTransformer(model_name, device=device)
    return _model_cache[model_name], model_name

def buscar_noticias_semantica(
    conn,
    page: int,
    per_page: int,
    q: str,
    categoria_id: Optional[str] = None,
    continente_id: Optional[str] = None,
    pais_id: Optional[str] = None,
    fecha: Optional[str] = None,
    lang: str = "es",
) -> Tuple[List[Dict], int, int, Dict]:
    """
    Búsqueda semántica usando embeddings y similitud coseno (vía producto punto si están normalizados).
    """
    if not q.strip():
        return buscar_noticias(conn, page, per_page, "", categoria_id, continente_id, pais_id, fecha, lang)

    offset = (page - 1) * per_page
    model, model_name = _get_emb_model()
    
    # Generar embedding de la consulta
    q_emb = model.encode([q], normalize_embeddings=True)[0].tolist()

    where = ["t.status = 'done'", "t.lang_to = %s"]
    params = [lang]

    if fecha:
        where.append("n.fecha::date = %s")
        params.append(fecha)
    if categoria_id:
        where.append("n.categoria_id = %s")
        params.append(int(categoria_id))
    if pais_id:
        where.append("n.pais_id = %s")
        params.append(int(pais_id))
    elif continente_id:
        where.append("p.continente_id = %s")
        params.append(int(continente_id))

    where_sql = " AND ".join(where)

    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        # Consulta de búsqueda vectorial (usamos un array_agg o similar para el producto punto si no hay pgvector)
        # Nota: Aquí asumo que usamos producto punto entre arrays de double precision
        query_sql = f"""
            WITH similarity AS (
                SELECT 
                    te.traduccion_id,
                    (
                        SELECT SUM(a*b)
                        FROM unnest(te.embedding, %s::double precision[]) AS t(a,b)
                    ) AS score
                FROM traduccion_embeddings te
                WHERE te.model = %s
            )
            SELECT 
                n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url, n.fuente_nombre,
                c.nombre AS categoria, p.nombre AS pais,
                t.id AS traduccion_id, t.titulo_trad AS titulo_traducido, t.resumen_trad AS resumen_traducido,
                TRUE AS tiene_traduccion, s.score
            FROM similarity s
            JOIN traducciones t ON t.id = s.traduccion_id
            JOIN noticias n ON n.id = t.noticia_id
            LEFT JOIN categorias c ON c.id = n.categoria_id
            LEFT JOIN paises p ON p.id = n.pais_id
            WHERE {where_sql}
            ORDER BY n.fecha DESC NULLS LAST, s.score DESC
            LIMIT %s OFFSET %s
        """
        
        # Para el conteo total en semántica podemos simplificar o usar el mismo WHERE
        cur.execute(f"SELECT COUNT(*) FROM traducciones t JOIN noticias n ON n.id = t.noticia_id LEFT JOIN paises p ON p.id = n.pais_id WHERE {where_sql}", params)
        total_results = cur.fetchone()[0]
        total_pages = (total_results // per_page) + (1 if total_results % per_page else 0)

        cur.execute(query_sql, [q_emb, model_name] + params + [per_page, offset])
        noticias = cur.fetchall()

        tr_ids = [n["traduccion_id"] for n in noticias]
        tags_por_tr = _extraer_tags_por_traduccion(cur, tr_ids)

    return noticias, total_results, total_pages, tags_por_tr
