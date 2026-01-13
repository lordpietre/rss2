"""
Favorites router - Save and manage favorite news.
"""
from flask import Blueprint, request, jsonify, session, render_template
from psycopg2 import extras
from db import get_read_conn, get_write_conn
from utils.auth import get_current_user, is_authenticated
import secrets

favoritos_bp = Blueprint("favoritos", __name__, url_prefix="/favoritos")


def get_user_or_session_id():
    """Get user ID if authenticated, otherwise session ID.
    
    Returns:
        Tuple of (user_id, session_id)
    """
    user = get_current_user()
    if user:
        return (user['id'], None)
    
    # Anonymous user - use session_id
    if "user_session" not in session:
        session["user_session"] = secrets.token_hex(16)
    return (None, session["user_session"])


def ensure_favoritos_table(conn):
    """Create/update favoritos table to support both users and sessions."""
    with conn.cursor() as cur:
        # Table is created by init-db scripts, just ensure it exists
        cur.execute("""
            CREATE TABLE IF NOT EXISTS favoritos (
                id SERIAL PRIMARY KEY,
                user_id INTEGER REFERENCES usuarios(id) ON DELETE CASCADE,
                session_id VARCHAR(64),
                noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
                created_at TIMESTAMP DEFAULT NOW()
            );
        """)
        cur.execute("CREATE INDEX IF NOT EXISTS idx_favoritos_session ON favoritos(session_id);")
        cur.execute("CREATE INDEX IF NOT EXISTS idx_favoritos_user_id ON favoritos(user_id);")
        
        # Ensure session_id can be null (for logged in users)
        try:
            cur.execute("ALTER TABLE favoritos ALTER COLUMN session_id DROP NOT NULL;")
        except Exception:
            conn.rollback()
        else:
            conn.commit()
    conn.commit()


# ============================================================
#   API: Toggle Favorite
# ============================================================

@favoritos_bp.route("/toggle/<noticia_id>", methods=["POST"])
def toggle_favorite(noticia_id):
    """Toggle favorite status for a news item."""
    user_id, session_id = get_user_or_session_id()
    
    with get_write_conn() as conn:
        ensure_favoritos_table(conn)
        
        with conn.cursor() as cur:
            # Check if already favorited (by user_id OR session_id)
            if user_id:
                cur.execute(
                    "SELECT id FROM favoritos WHERE user_id = %s AND noticia_id = %s",
                    (user_id, noticia_id)
                )
            else:
                cur.execute(
                    "SELECT id FROM favoritos WHERE session_id = %s AND noticia_id = %s",
                    (session_id, noticia_id)
                )
            existing = cur.fetchone()
            
            if existing:
                # Remove favorite
                if user_id:
                    cur.execute(
                        "DELETE FROM favoritos WHERE user_id = %s AND noticia_id = %s",
                        (user_id, noticia_id)
                    )
                else:
                    cur.execute(
                        "DELETE FROM favoritos WHERE session_id = %s AND noticia_id = %s",
                        (session_id, noticia_id)
                    )
                is_favorite = False
            else:
                # Add favorite
                cur.execute(
                    "INSERT INTO favoritos (user_id, session_id, noticia_id) VALUES (%s, %s, %s) ON CONFLICT DO NOTHING",
                    (user_id, session_id, noticia_id)
                )
                is_favorite = True
        
        conn.commit()
    
    return jsonify({"success": True, "is_favorite": is_favorite})


# ============================================================
#   API: Check if Favorite
# ============================================================

@favoritos_bp.route("/check/<noticia_id>")
def check_favorite(noticia_id):
    """Check if a news item is favorited."""
    user_id, session_id = get_user_or_session_id()
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            if user_id:
                cur.execute(
                    "SELECT id FROM favoritos WHERE user_id = %s AND noticia_id = %s",
                    (user_id, noticia_id)
                )
            else:
                cur.execute(
                    "SELECT id FROM favoritos WHERE session_id = %s AND noticia_id = %s",
                    (session_id, noticia_id)
                )
            is_favorite = cur.fetchone() is not None
    
    return jsonify({"is_favorite": is_favorite})


# ============================================================
#   API: Get User's Favorites IDs
# ============================================================

@favoritos_bp.route("/ids")
def get_favorite_ids():
    """Get list of favorite noticia IDs for current user."""
    user_id, session_id = get_user_or_session_id()
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            if user_id:
                cur.execute(
                    "SELECT noticia_id FROM favoritos WHERE user_id = %s",
                    (user_id,)
                )
            else:
                cur.execute(
                    "SELECT noticia_id FROM favoritos WHERE session_id = %s",
                    (session_id,)
                )
            ids = [row[0] for row in cur.fetchall()]
    
    return jsonify({"ids": ids})


# ============================================================
#   Page: View Favorites
# ============================================================

@favoritos_bp.route("/")
def view_favorites():
    """View all favorited news items."""
    user_id, session_id = get_user_or_session_id()
    user = get_current_user()
    
    with get_read_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            if user_id:
                cur.execute("""
                    SELECT n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url,
                           n.fuente_nombre, c.nombre AS categoria, p.nombre AS pais,
                           t.titulo_trad, t.resumen_trad, t.lang_to,
                           f.created_at AS favorito_at
                    FROM favoritos f
                    JOIN noticias n ON n.id = f.noticia_id
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es' AND t.status = 'done'
                    WHERE f.user_id = %s
                    ORDER BY f.created_at DESC
                    LIMIT 100;
                """, (user_id,))
            else:
                cur.execute("""
                    SELECT n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url,
                           n.fuente_nombre, c.nombre AS categoria, p.nombre AS pais,
                           t.titulo_trad, t.resumen_trad, t.lang_to,
                           f.created_at AS favorito_at
                    FROM favoritos f
                    JOIN noticias n ON n.id = f.noticia_id
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es' AND t.status = 'done'
                    WHERE f.session_id = %s
                    ORDER BY f.created_at DESC
                    LIMIT 100;
                """, (session_id,))
            noticias = cur.fetchall()
    
    return render_template("favoritos.html", noticias=noticias, user=user)
