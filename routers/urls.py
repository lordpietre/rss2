from flask import Blueprint, render_template, request, redirect, flash, url_for
from psycopg2 import extras
from db import get_conn
from models.categorias import get_categorias
from models.paises import get_paises

urls_bp = Blueprint("urls", __name__, url_prefix="/urls")


@urls_bp.route("/")
def manage_urls():
    with get_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT fu.id, fu.nombre, fu.url,
                   c.nombre AS categoria,
                   p.nombre AS pais,
                   fu.idioma,
                   fu.last_check,
                   fu.last_status,
                   fu.status_message,
                   fu.last_http_code,
                   COALESCE((
                       SELECT COUNT(*)
                       FROM noticias n
                       JOIN feeds f ON n.fuente_nombre = f.nombre
                       WHERE f.fuente_url_id = fu.id
                   ), 0) as noticias_count
            FROM fuentes_url fu
            LEFT JOIN categorias c ON c.id=fu.categoria_id
            LEFT JOIN paises p ON p.id=fu.pais_id
            ORDER BY fu.nombre;
            """
        )
        fuentes = cur.fetchall()

    return render_template("urls_list.html", fuentes=fuentes)


@urls_bp.route("/add_source", methods=["GET", "POST"])
def add_url_source():
    with get_conn() as conn:
        categorias = get_categorias(conn)
        paises = get_paises(conn)

        if request.method == "POST":
            nombre = request.form.get("nombre")
            url = request.form.get("url")
            categoria_id = request.form.get("categoria_id")
            pais_id = request.form.get("pais_id")
            idioma = (request.form.get("idioma", "es") or "es").strip().lower()[:2]

            try:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO fuentes_url (nombre, url, categoria_id, pais_id, idioma)
                        VALUES (%s, %s, %s, %s, %s)
                        ON CONFLICT (url) DO UPDATE
                           SET nombre=EXCLUDED.nombre,
                               categoria_id=EXCLUDED.categoria_id,
                               pais_id=EXCLUDED.pais_id,
                               idioma=EXCLUDED.idioma;
                        """,
                        (
                            nombre,
                            url,
                            int(categoria_id) if categoria_id else None,
                            int(pais_id) if pais_id else None,
                            idioma,
                        ),
                    )
                conn.commit()
                flash("Fuente añadida/actualizada.", "success")
                return redirect(url_for("urls.manage_urls"))

            except Exception as e:
                flash(f"Error: {e}", "error")

    return render_template("add_url_source.html", categorias=categorias, paises=paises)

