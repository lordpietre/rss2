from flask import Blueprint, render_template, request, flash, redirect, url_for
from db import get_conn, get_read_conn
import psycopg2.extras
from utils.qdrant_search import search_by_keywords

conflicts_bp = Blueprint("conflicts", __name__, url_prefix="/conflicts")

def ensure_table(conn):
    with conn.cursor() as cur:
        cur.execute("""
            CREATE TABLE IF NOT EXISTS conflicts (
                id SERIAL PRIMARY KEY,
                name VARCHAR(100) NOT NULL,
                keywords TEXT,
                description TEXT,
                created_at TIMESTAMP DEFAULT NOW()
            );
        """)
    conn.commit()

@conflicts_bp.route("/")
def index():
    with get_conn() as conn:
        ensure_table(conn)
        with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
            cur.execute("SELECT * FROM conflicts ORDER BY id DESC")
            conflicts = cur.fetchall()
            
    return render_template("conflicts_list.html", conflicts=conflicts)

@conflicts_bp.route("/create", methods=["POST"])
def create():
    name = request.form.get("name")
    keywords = request.form.get("keywords")
    description = request.form.get("description", "")
    
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO conflicts (name, keywords, description) VALUES (%s, %s, %s)",
                (name, keywords, description)
            )
        conn.commit()
    
    flash("Conflicto creado correctamente.", "success")
    return redirect(url_for("conflicts.index"))

@conflicts_bp.route("/<int:id>")
def timeline(id):
    with get_conn() as conn:
        with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
            cur.execute("SELECT * FROM conflicts WHERE id = %s", (id,))
            conflict = cur.fetchone()
            
            if not conflict:
                flash("Conflicto no encontrado.", "error")
                return redirect(url_for("conflicts.index"))
    
    # Keywords logic: comma separated
    kw_raw = conflict['keywords'] or ""
    kw_list = [k.strip() for k in kw_raw.split(',') if k.strip()]
    
    noticias = []
    
    if kw_list:
        try:
            # Usar búsqueda semántica por keywords (mucho más rápido y efectivo)
            semantic_results = search_by_keywords(
                keywords=kw_list,
                limit=200,
                score_threshold=0.35
            )
            
            # Enriquecer con datos de PostgreSQL
            if semantic_results:
                news_ids = [r['news_id'] for r in semantic_results]
                
                with get_read_conn() as conn:
                    with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                        cur.execute("""
                            SELECT 
                                t.id AS tr_id,
                                t.lang_to,
                                COALESCE(t.titulo_trad, n.titulo) as titulo,
                                COALESCE(t.resumen_trad, n.resumen) as resumen,
                                n.id AS noticia_id,
                                n.fecha,
                                n.imagen_url,
                                n.fuente_nombre,
                                p.nombre as pais
                            FROM noticias n
                            LEFT JOIN traducciones t ON n.id = t.noticia_id AND t.lang_to = 'es'
                            LEFT JOIN paises p ON p.id = n.pais_id
                            WHERE n.id = ANY(%s)
                            ORDER BY n.fecha DESC
                        """, (news_ids,))
                        
                        noticias = cur.fetchall()
                        
        except Exception as e:
            print(f"⚠️ Error en búsqueda semántica de conflictos, usando fallback: {e}")
            
            # Fallback a búsqueda tradicional ILIKE
            patterns = [f"%{k}%" for k in kw_list]
            
            with get_read_conn() as conn:
                with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                    cur.execute("""
                        SELECT 
                            t.id AS tr_id,
                            t.lang_to,
                            COALESCE(t.titulo_trad, n.titulo) as titulo,
                            COALESCE(t.resumen_trad, n.resumen) as resumen,
                            n.id AS noticia_id,
                            n.fecha,
                            n.imagen_url,
                            n.fuente_nombre,
                            p.nombre as pais
                        FROM noticias n
                        LEFT JOIN traducciones t ON n.id = t.noticia_id AND t.lang_to = 'es'
                        LEFT JOIN paises p ON p.id = n.pais_id
                        WHERE 
                            (t.titulo_trad ILIKE ANY(%s) OR n.titulo ILIKE ANY(%s))
                            OR
                            (t.resumen_trad ILIKE ANY(%s) OR n.resumen ILIKE ANY(%s))
                        ORDER BY n.fecha DESC
                        LIMIT 200
                    """, (patterns, patterns, patterns, patterns))
                    
                    noticias = cur.fetchall()
                  
    return render_template("conflict_timeline.html", conflict=conflict, noticias=noticias)

@conflicts_bp.route("/delete/<int:id>", methods=["POST"])
def delete(id):
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("DELETE FROM conflicts WHERE id = %s", (id,))
        conn.commit()
    flash("Conflicto eliminado.", "success")
    return redirect(url_for("conflicts.index"))
