from flask import Blueprint, render_template, request
from datetime import datetime
from psycopg2 import extras
from db import get_read_conn, get_write_conn
from utils.auth import get_current_user
from config import DEFAULT_TRANSLATION_LANG, DEFAULT_LANG, NEWS_PER_PAGE_DEFAULT
from models.categorias import get_categorias
from models.paises import get_paises
from models.noticias import buscar_noticias

home_bp = Blueprint("home", __name__)

@home_bp.route("/")
@home_bp.route("/home")
def home():
    """Simplified home page to avoid timeouts."""
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
    fecha_filtro = None
    if fecha_str:
        try:
            fecha_filtro = datetime.strptime(fecha_str, "%Y-%m-%d").date()
        except ValueError:
            fecha_filtro = None

    # Búsqueda semántica solo si se solicita explícitamente y hay query
    use_semantic = bool(request.args.get("semantic")) and bool(q)
    
    with get_read_conn() as conn:
        conn.autocommit = True
        categorias = get_categorias(conn)
        paises = get_paises(conn)

        if use_semantic:
            from models.noticias import buscar_noticias_semantica
            noticias, total_results, total_pages, tags_por_tr = buscar_noticias_semantica(
                conn=conn,
                page=page,
                per_page=per_page,
                q=q,
                categoria_id=categoria_id,
                continente_id=continente_id,
                pais_id=pais_id,
                fecha=fecha_filtro,
                lang=lang,
            )
        else:
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

        # Historial de búsqueda (solo para usuarios logueados y en primera página)
        recent_searches_with_results = []
        user = get_current_user()
        if user and page == 1 and not q:
             with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                 cur.execute("""
                     SELECT sh.id, sh.query, sh.searched_at, sh.results_count,
                            p.nombre as pais_nombre, c.nombre as categoria_nombre
                     FROM search_history sh
                     LEFT JOIN paises p ON p.id = sh.pais_id
                     LEFT JOIN categorias c ON c.id = sh.categoria_id
                     WHERE sh.user_id = %s
                     ORDER BY sh.searched_at DESC
                     LIMIT 10
                 """, (user['id'],))
                 recent_searches_with_results = cur.fetchall()

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
        cont_id=int(continente_id) if continente_id else None,
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
                cur.execute(
                    "DELETE FROM search_history WHERE id = %s AND user_id = %s",
                    (search_id, user["id"])
                )
            conn.commit()
        return {"success": True}
    except Exception as e:
        print(f"Error deleting search {search_id}: {e}")
        return {"error": str(e)}, 500
