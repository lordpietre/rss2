"""
Resumen router - Daily summary of news.
"""
from flask import Blueprint, render_template, request
from psycopg2 import extras
from db import get_conn
from datetime import datetime, timedelta

resumen_bp = Blueprint("resumen", __name__, url_prefix="/resumen")


@resumen_bp.route("/")
def diario():
    """Daily summary page."""
    # Default to today
    date_str = request.args.get("date")
    if date_str:
        try:
            target_date = datetime.strptime(date_str, "%Y-%m-%d").date()
        except ValueError:
            target_date = datetime.utcnow().date()
    else:
        target_date = datetime.utcnow().date()

    prev_date = target_date - timedelta(days=1)
    next_date = target_date + timedelta(days=1)
    
    if next_date > datetime.utcnow().date():
        next_date = None

    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Fetch top news for the day grouped by category
            # We'll limit to 5 per category to keep it concise
            cur.execute("""
                WITH ranked_news AS (
                    SELECT 
                        n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url, n.fuente_nombre,
                        c.id as cat_id, c.nombre as categoria,
                        t.titulo_trad, t.resumen_trad,
                        ROW_NUMBER() OVER (PARTITION BY n.categoria_id ORDER BY n.fecha DESC) as rn
                    FROM noticias n
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN traducciones t ON t.noticia_id = n.id 
                        AND t.lang_to = 'es' AND t.status = 'done'
                    WHERE n.fecha >= %s AND n.fecha < %s + INTERVAL '1 day'
                )
                SELECT * FROM ranked_news WHERE rn <= 5 ORDER BY categoria, rn
            """, (target_date, target_date))
            
            rows = cur.fetchall()

    # Group by category
    noticias_by_cat = {}
    for r in rows:
        cat = r["categoria"] or "Sin Categoría"
        if cat not in noticias_by_cat:
            noticias_by_cat[cat] = []
        
        noticias_by_cat[cat].append({
            "id": r["id"],
            "titulo": r["titulo_trad"] or r["titulo"],
            "resumen": r["resumen_trad"] or r["resumen"],
            "url": r["url"],
            "fecha": r["fecha"],
            "imagen_url": r["imagen_url"],
            "fuente": r["fuente_nombre"]
        })

    return render_template(
        "resumen.html", 
        noticias_by_cat=noticias_by_cat,
        current_date=target_date,
        prev_date=prev_date,
        next_date=next_date
    )
