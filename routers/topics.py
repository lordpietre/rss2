from flask import Blueprint, render_template, request
from db import get_read_conn
from psycopg2 import extras
import datetime

topics_bp = Blueprint("topics", __name__, url_prefix="/topics")

@topics_bp.route("/")
def monitor():
    # Monitor de Impacto por País
    days = int(request.args.get("days", 3))
    
    with get_read_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:
        # Ranking de Países por "Calor" (Suma de scores de noticias recientes)
        cur.execute("""
            SELECT p.id, p.nombre, 
                   COUNT(DISTINCT n.id) as news_count,
                   SUM(nt.score) as total_impact
            FROM paises p
            JOIN noticias n ON n.pais_id = p.id
            JOIN news_topics nt ON nt.noticia_id = n.id
            WHERE n.fecha > NOW() - INTERVAL '%s days'
            GROUP BY p.id, p.nombre
            HAVING SUM(nt.score) > 0
            ORDER BY total_impact DESC
            LIMIT 50;
        """, (days,))
        countries = cur.fetchall()
        
    return render_template("monitor_list.html", countries=countries, days=days)

@topics_bp.route("/country/<int:pais_id>")
def country_detail(pais_id):
    days = int(request.args.get("days", 3))
    
    with get_read_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:
        # Info País
        cur.execute("SELECT * FROM paises WHERE id = %s", (pais_id,))
        pais = cur.fetchone()
        
        if not pais:
            return "País no encontrado", 404
            
        # Top Noticias de Impacto (Últimos días)
        # News with their highest topic score sum
        cur.execute("""
            SELECT n.id, 
                   COALESCE(t.titulo_trad, n.titulo) as titulo, 
                   COALESCE(t.resumen_trad, n.resumen) as resumen, 
                   n.fecha, n.imagen_url, n.fuente_nombre, n.url,
                   SUM(nt.score) as impact_score
            FROM noticias n
            JOIN news_topics nt ON nt.noticia_id = n.id
            LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es' AND t.status = 'done'
            WHERE n.pais_id = %s
              AND n.fecha > NOW() - INTERVAL '%s days'
            GROUP BY n.id, n.titulo, n.resumen, n.fecha, n.imagen_url, n.fuente_nombre, n.url, t.titulo_trad, t.resumen_trad
            ORDER BY impact_score DESC
            LIMIT 20;
        """, (pais_id, days))
        top_news = cur.fetchall()
        
        # Temas Activos en este País
        cur.execute("""
            SELECT t.name, SUM(nt.score) as topic_volume
            FROM topics t
            JOIN news_topics nt ON nt.topic_id = t.id
            JOIN noticias n ON n.id = nt.noticia_id
            WHERE n.pais_id = %s
              AND n.fecha > NOW() - INTERVAL '%s days'
            GROUP BY t.id, t.name
            ORDER BY topic_volume DESC
            LIMIT 10;
        """, (pais_id, days))
        active_topics = cur.fetchall()
        
    return render_template("monitor_detail.html", 
                         pais=pais, 
                         news=top_news, 
                         active_topics=active_topics,
                         days=days)
