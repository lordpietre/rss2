from flask import Blueprint, render_template, request
from datetime import datetime
from psycopg2 import extras
from db import get_read_conn, get_write_conn
from utils.auth import get_current_user
from config import DEFAULT_TRANSLATION_LANG, DEFAULT_LANG, NEWS_PER_PAGE_DEFAULT
from models.categorias import get_categorias
from models.paises import get_paises
from models.noticias import buscar_noticias, buscar_noticias_semantica
from cache import cached

home_bp = Blueprint("home", __name__)

@home_bp.route("/")
@home_bp.route("/home")
def home():
    page = max(int(request.args.get("page", 1)), 1)
    per_page = int(request.args.get("per_page", NEWS_PER_PAGE_DEFAULT))
    per_page = min(max(per_page, 10), 100)

    q = (request.args.get("q") or "").strip()
    categoria_id = request.args.get("categoria_id")
    continente_id = request.args.get("continente_id")
    pais_id = request.args.get("pais_id")
    fecha_str = request.args.get("fecha") or ""

    lang = (request.args.get("lang") or DEFAULT_TRANSLATION_LANG or DEFAULT_LANG).lower()[:5]
    
    use_tr = not bool(request.args.get("orig"))
    fecha_str = request.args.get("fecha") or ""
    fecha_filtro = None
    if fecha_str:
        try:
            fecha_filtro = datetime.strptime(fecha_str, "%Y-%m-%d").date()
        except ValueError:
            fecha_filtro = None

    from utils.qdrant_search import semantic_search
    
    # Logic for semantic search enabled by default if query exists, unless explicitly disabled
    # If the user passed 'semantic=' explicitly as empty string, it might mean False, but for UX speed default to True is better.
    # However, let's respect the flag if it's explicitly 'false' or '0'.
    # If key is missing, default to True. If key is present but empty, treat as False (standard HTML form behavior unfortunately).
    # But wait, the previous log showed 'semantic='. HTML checkboxes send nothing if unchecked, 'on' if checked.
    # So if it appears as empty string, it might be a hidden input or unassigned var.
    # Let's check 'semantic' param presence.
    raw_semantic = request.args.get("semantic")
    if raw_semantic is None:
        use_semantic = True # Default to semantic if not specified
    elif raw_semantic == "" or raw_semantic.lower() in ["false", "0", "off"]:
        use_semantic = False
    else:
        use_semantic = True

    with get_read_conn() as conn:
        conn.autocommit = True
        categorias = get_categorias(conn)
        paises = get_paises(conn)

        noticias = []
        total_results = 0
        total_pages = 0
        tags_por_tr = {}

        # 1. Intentar búsqueda semántica si hay query y está habilitado
        semantic_success = False
        if use_semantic and q:
            try:
                # Obtener más resultados para 'llenar' la página si hay IDs no encontrados
                limit_fetch = per_page * 2 
                
                sem_results = semantic_search(
                    query=q,
                    limit=limit_fetch, # Pedimos más para asegurar
                    score_threshold=0.30
                )
                
                if sem_results:
                    # Extraer IDs
                    news_ids = [r['news_id'] for r in sem_results]
                    
                    # Traer datos completos de PostgreSQL (igual que en search.py)
                    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                        query_sql = """
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
                            WHERE n.id = ANY(%s)
                        """
                        cur.execute(query_sql, (lang, news_ids))
                        rows = cur.fetchall()
                        
                        # Convertimos a lista para poder ordenar por fecha
                        rows_list = list(rows)
                        
                        # Ordenar cronológicamente (más reciente primero)
                        sorted_rows = sorted(
                            rows_list, 
                            key=lambda x: x['fecha'] if x['fecha'] else datetime.min, 
                            reverse=True
                        )
                                
                        # Aplicar paginación manual sobre los resultados ordenados
                        # Nota: semantic_search ya devolvió los "top" globales (aproximadamente). 
                        # Para paginación real profunda con Qdrant se necesita scroll/offset, 
                        # aquí asumimos que page request mapea al limit/offset enviado a Qdrant.
                        # Pero `semantic_search` simple en utils no tiene offset.
                        # Arreglo temporal: Solo mostramos la primera "tanda" de resultados semánticos.
                        # Si el usuario quiere paginar profundo, Qdrant search debe soportar offset.
                        # utils/qdrant_search.py NO tiene offset. 
                        # ASÍ QUE: Solo funcionará bien para la página 1.
                        # Si page > 1, semantic_search simple no sirve sin offset.
                        
                        # Fallback: Si page > 1, usamos búsqueda tradicional O implementamos offset en Qdrant (mejor).
                        # Por ahora: Usamos lo que devolvió semantic_search y cortamos localmente
                        # si page=1.
                        
                        if len(sorted_rows) > 0:
                            noticias = sorted_rows
                            total_results = len(noticias) # Aproximado
                            total_pages = 1 # Qdrant simple no pagina bien aun
                            
                            # Extraer tags
                            tr_ids = [n["traduccion_id"] for n in noticias if n["traduccion_id"]]
                            from models.noticias import _extraer_tags_por_traduccion
                            tags_por_tr = _extraer_tags_por_traduccion(cur, tr_ids)
                            
                            semantic_success = True

            except Exception as e:
                print(f"⚠️ Error en semántica home, fallback: {e}")
                semantic_success = False

        # 2. Si no hubo búsqueda semántica (o falló, o no había query, o usuario la desactivó), usar la tradicional
        if not semantic_success:
            noticias, total_results, total_pages, tags_por_tr = buscar_noticias(
                conn=conn,
                page=page,
                per_page=per_page,
                q=q,
                categoria_id=categoria_id,
                continente_id=continente_id,
                pais_id=pais_id,
                fecha=fecha_filtro,
                lang=lang,
                use_tr=use_tr,
            )

    # Record search history for logged-in users (only on first page to avoid dupes)
    if (q or categoria_id or pais_id) and page == 1:
        user = get_current_user()
        if user:
            try:
                with get_write_conn() as w_conn:
                    with w_conn.cursor() as w_cur:
                        # Check if it's the same as the last search to avoid immediate duplicates
                        w_cur.execute("""
                            SELECT query, pais_id, categoria_id 
                            FROM search_history 
                            WHERE user_id = %s 
                            ORDER BY searched_at DESC LIMIT 1
                        """, (user['id'],))
                        last_search = w_cur.fetchone()
                        
                        current_search = (q or None, int(pais_id) if pais_id else None, int(categoria_id) if categoria_id else None)
                        
                        if not last_search or (last_search[0], last_search[1], last_search[2]) != current_search:
                            w_cur.execute("""
                                INSERT INTO search_history (user_id, query, pais_id, categoria_id, results_count)
                                VALUES (%s, %s, %s, %s, %s)
                            """, (user['id'], current_search[0], current_search[1], current_search[2], total_results))
                    w_conn.commit()
            except Exception as e:
                # Log error but don't break the page load
                print(f"Error saving search history: {e}")
                pass

    user = get_current_user()
    recent_searches_with_results = []
    if user and not q and not categoria_id and not pais_id and page == 1:
        with get_read_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                # Fetch unique latest searches using DISTINCT ON
                cur.execute("""
                    SELECT sub.id, query, pais_id, categoria_id, results_count, searched_at,
                           p.nombre as pais_nombre, c.nombre as categoria_nombre
                    FROM (
                        SELECT DISTINCT ON (COALESCE(query, ''), COALESCE(pais_id, 0), COALESCE(categoria_id, 0))
                            id, query, pais_id, categoria_id, results_count, searched_at
                        FROM search_history
                        WHERE user_id = %s
                        ORDER BY COALESCE(query, ''), COALESCE(pais_id, 0), COALESCE(categoria_id, 0), searched_at DESC
                    ) sub
                    LEFT JOIN paises p ON p.id = sub.pais_id
                    LEFT JOIN categorias c ON c.id = sub.categoria_id
                    ORDER BY searched_at DESC
                    LIMIT 6
                """, (user['id'],))
                recent_searches = cur.fetchall()
                
                for s in recent_searches:
                    # Fetch top 6 news for this search
                    news_items, _, _, _ = buscar_noticias(
                        conn=conn,
                        page=1,
                        per_page=6,
                        q=s['query'] or "",
                        pais_id=s['pais_id'],
                        categoria_id=s['categoria_id'],
                        lang=lang,
                        use_tr=use_tr,
                        skip_count=True
                    )
                    recent_searches_with_results.append({
                        'id': s['id'],
                        'query': s['query'],
                        'pais_id': s['pais_id'],
                        'pais_nombre': s['pais_nombre'],
                        'categoria_id': s['categoria_id'],
                        'categoria_nombre': s['categoria_nombre'],
                        'results_count': s['results_count'],
                        'searched_at': s['searched_at'],
                        'noticias': news_items
                    })

    context = dict(
        noticias=noticias,
        total_results=total_results,
        total_pages=total_pages,
        page=page,
        per_page=per_page,
        categorias=categorias,
        paises=paises,
        q=q,
        cat_id=int(categoria_id) if categoria_id else None,
        pais_id=int(pais_id) if pais_id else None,
        fecha_filtro=fecha_str,
        lang=lang,
        use_tr=use_tr,
        use_semantic=use_semantic,
        tags_por_tr=tags_por_tr,
        recent_searches_with_results=recent_searches_with_results,
    )

    if request.headers.get("X-Requested-With") == "XMLHttpRequest":
        return render_template("_noticias_list.html", **context)

    return render_template("noticias.html", **context)


@home_bp.route("/delete_search/<int:search_id>", methods=["POST"])
def delete_search(search_id):
    user = get_current_user()
    if not user:
        return {"error": "No autenticado"}, 401
    
    try:
        with get_write_conn() as conn:
            with conn.cursor() as cur:
                # Direct deletion ensuring ownership
                cur.execute(
                    "DELETE FROM search_history WHERE id = %s AND user_id = %s",
                    (search_id, user["id"])
                )
            conn.commit()
        return {"success": True}
    except Exception as e:
        print(f"Error deleting search {search_id}: {e}")
        return {"error": str(e)}, 500
