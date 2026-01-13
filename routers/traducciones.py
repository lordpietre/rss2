from flask import Blueprint, render_template, request
from db import get_read_conn

traducciones_bp = Blueprint("traducciones", __name__)


@traducciones_bp.route("/traducciones")
def ultimas_traducciones():
    """Muestra las últimas noticias traducidas."""
    page = max(int(request.args.get("page", 1)), 1)
    per_page = min(max(int(request.args.get("per_page", 20)), 10), 100)
    offset = (page - 1) * per_page

    with get_read_conn() as conn:
        conn.autocommit = True
        with conn.cursor() as cur:
            # Total count
            cur.execute("""
                SELECT COUNT(*) FROM traducciones WHERE status = 'done'
            """)
            total = cur.fetchone()[0]

            # Fetch latest translations
            cur.execute("""
                SELECT 
                    t.id,
                    t.noticia_id,
                    t.titulo_trad,
                    t.resumen_trad,
                    t.lang_from,
                    t.lang_to,
                    t.created_at AS updated_at,
                    n.url AS link,
                    n.imagen_url AS imagen,
                    n.fuente_nombre AS feed_nombre,
                    c.nombre AS categoria_nombre,
                    p.nombre AS pais_nombre
                FROM traducciones t
                JOIN noticias n ON n.id = t.noticia_id
                LEFT JOIN categorias c ON c.id = n.categoria_id
                LEFT JOIN paises p ON p.id = n.pais_id
                WHERE t.status = 'done'
                ORDER BY t.created_at DESC
                LIMIT %s OFFSET %s
            """, (per_page, offset))
            
            columns = [desc[0] for desc in cur.description]
            traducciones = [dict(zip(columns, row)) for row in cur.fetchall()]

    total_pages = (total + per_page - 1) // per_page

    return render_template(
        "traducciones.html",
        traducciones=traducciones,
        page=page,
        per_page=per_page,
        total=total,
        total_pages=total_pages,
    )
