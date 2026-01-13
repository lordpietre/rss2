"""
RSS Feed router - Generate custom RSS feeds with filters.
"""
from flask import Blueprint, request, Response
from psycopg2 import extras
from db import get_read_conn
from datetime import datetime
import html

rss_bp = Blueprint("rss", __name__, url_prefix="/rss")


def escape_xml(text):
    """Escape text for XML."""
    if not text:
        return ""
    return html.escape(str(text))


def build_rss_xml(title, description, link, items):
    """Build RSS 2.0 XML feed."""
    now = datetime.utcnow().strftime("%a, %d %b %Y %H:%M:%S +0000")
    
    xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
    <title>{escape_xml(title)}</title>
    <description>{escape_xml(description)}</description>
    <link>{escape_xml(link)}</link>
    <lastBuildDate>{now}</lastBuildDate>
    <language>es</language>
'''
    
    for item in items:
        pub_date = ""
        if item.get("fecha"):
            try:
                pub_date = item["fecha"].strftime("%a, %d %b %Y %H:%M:%S +0000")
            except:
                pass
        
        xml += f'''    <item>
        <title>{escape_xml(item.get("titulo", ""))}</title>
        <description><![CDATA[{item.get("resumen", "")}]]></description>
        <link>{escape_xml(item.get("url", ""))}</link>
        <guid isPermaLink="false">{escape_xml(item.get("id", ""))}</guid>
        <pubDate>{pub_date}</pubDate>
    </item>
'''
    
    xml += '''</channel>
</rss>'''
    
    return xml


@rss_bp.route("/custom")
def custom_feed():
    """
    Generate a custom RSS feed with filters.
    
    Query params:
    - pais_id: Filter by country ID
    - categoria_id: Filter by category ID
    - lang: Translation language (default: es)
    - limit: Number of items (default: 50, max: 100)
    """
    pais_id = request.args.get("pais_id")
    categoria_id = request.args.get("categoria_id")
    lang = (request.args.get("lang") or "es").lower()[:5]
    limit = min(int(request.args.get("limit", 50)), 100)
    
    # Build description based on filters
    filters_desc = []
    
    with get_read_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Get filter names for description
            if pais_id:
                cur.execute("SELECT nombre FROM paises WHERE id = %s", (pais_id,))
                row = cur.fetchone()
                if row:
                    filters_desc.append(f"País: {row['nombre']}")
            
            if categoria_id:
                cur.execute("SELECT nombre FROM categorias WHERE id = %s", (categoria_id,))
                row = cur.fetchone()
                if row:
                    filters_desc.append(f"Categoría: {row['nombre']}")
            
            # Build query
            query = """
                SELECT 
                    n.id, n.titulo, n.resumen, n.url, n.fecha,
                    n.imagen_url, n.fuente_nombre,
                    t.titulo_trad, t.resumen_trad
                FROM noticias n
                LEFT JOIN traducciones t ON t.noticia_id = n.id 
                    AND t.lang_to = %s AND t.status = 'done'
                WHERE 1=1
            """
            params = [lang]
            
            if pais_id:
                query += " AND n.pais_id = %s"
                params.append(pais_id)
            
            if categoria_id:
                query += " AND n.categoria_id = %s"
                params.append(categoria_id)
            
            query += " ORDER BY n.fecha DESC LIMIT %s"
            params.append(limit)
            
            cur.execute(query, tuple(params))
            rows = cur.fetchall()
    
    # Build items
    items = []
    for r in rows:
        items.append({
            "id": r["id"],
            "titulo": r["titulo_trad"] or r["titulo"],
            "resumen": r["resumen_trad"] or r["resumen"] or "",
            "url": r["url"],
            "fecha": r["fecha"],
        })
    
    # Build feed metadata
    title = "The Daily Feed"
    if filters_desc:
        title += " - " + ", ".join(filters_desc)
    
    description = "Noticias personalizadas"
    if filters_desc:
        description = "Feed personalizado: " + ", ".join(filters_desc)
    
    link = request.host_url.rstrip("/")
    
    xml = build_rss_xml(title, description, link, items)
    
    return Response(xml, mimetype="application/rss+xml")


@rss_bp.route("/favoritos")
def favoritos_feed():
    """Generate RSS feed of user's favorites."""
    from routers.favoritos import get_session_id, ensure_favoritos_table
    
    session_id = get_session_id()
    
    with get_read_conn() as conn:
        ensure_favoritos_table(conn)
        
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            cur.execute("""
                SELECT n.id, n.titulo, n.resumen, n.url, n.fecha,
                       t.titulo_trad, t.resumen_trad
                FROM favoritos f
                JOIN noticias n ON n.id = f.noticia_id
                LEFT JOIN traducciones t ON t.noticia_id = n.id 
                    AND t.lang_to = 'es' AND t.status = 'done'
                WHERE f.session_id = %s
                ORDER BY f.created_at DESC
                LIMIT 50;
            """, (session_id,))
            rows = cur.fetchall()
    
    items = []
    for r in rows:
        items.append({
            "id": r["id"],
            "titulo": r["titulo_trad"] or r["titulo"],
            "resumen": r["resumen_trad"] or r["resumen"] or "",
            "url": r["url"],
            "fecha": r["fecha"],
        })
    
    xml = build_rss_xml(
        "The Daily Feed - Mis Favoritos",
        "Noticias guardadas en favoritos",
        request.host_url.rstrip("/"),
        items
    )
    
    return Response(xml, mimetype="application/rss+xml")
