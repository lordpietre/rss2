from flask import Blueprint, render_template, request, redirect, flash, url_for
from db import get_read_conn
from psycopg2 import extras

noticia_bp = Blueprint("noticia", __name__)


@noticia_bp.route("/noticia")
def noticia():
    tr_id = request.args.get("tr_id")
    noticia_id = request.args.get("id")

    if not tr_id and not noticia_id:
        flash("No se ha indicado ninguna noticia.", "warning")
        return redirect(url_for("home.home"))

    with get_read_conn() as conn:
        conn.autocommit = True
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            dato = None

            if tr_id:
                cur.execute(
                    """
                    SELECT
                        t.id AS traduccion_id,
                        t.lang_from,
                        t.lang_to,
                        t.titulo_trad,
                        t.resumen_trad,
                        n.id AS noticia_id,
                        n.titulo AS titulo_orig,
                        n.resumen AS resumen_orig,
                        n.url,
                        n.fecha,
                        n.imagen_url,
                        n.fuente_nombre,
                        c.nombre AS categoria,
                        p.nombre AS pais
                    FROM traducciones t
                    JOIN noticias n ON n.id = t.noticia_id
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    WHERE t.id = %s
                    """,
                    (int(tr_id),),
                )
                dato = cur.fetchone()

            else:
                cur.execute(
                    """
                    SELECT
                        NULL AS traduccion_id,
                        NULL AS lang_from,
                        NULL AS lang_to,
                        NULL AS titulo_trad,
                        NULL AS resumen_trad,
                        n.id AS noticia_id,
                        n.titulo AS titulo_orig,
                        n.resumen AS resumen_orig,
                        n.url,
                        n.fecha,
                        n.imagen_url,
                        n.fuente_nombre,
                        c.nombre AS categoria,
                        p.nombre AS pais
                    FROM noticias n
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    WHERE n.id = %s
                    """,
                    (noticia_id,),
                )
                dato = cur.fetchone()

            tags = []
            relacionadas = []

            if dato and dato["traduccion_id"]:
                cur.execute(
                    """
                    SELECT tg.valor, tg.tipo
                    FROM tags_noticia tn
                    JOIN tags tg ON tg.id = tn.tag_id
                    WHERE tn.traduccion_id = %s
                    ORDER BY tg.tipo, tg.valor;
                    """,
                    (dato["traduccion_id"],),
                )
                tags = cur.fetchall()

                cur.execute(
                    """
                    SELECT
                        n2.url,
                        n2.titulo,
                        n2.fecha,
                        n2.imagen_url,
                        n2.fuente_nombre,
                        rn.score,
                        t2.titulo_trad,
                        t2.id AS related_tr_id
                    FROM related_noticias rn
                    JOIN traducciones t2 ON t2.id = rn.related_traduccion_id
                    JOIN noticias n2 ON n2.id = t2.noticia_id
                    WHERE rn.traduccion_id = %s
                    ORDER BY rn.score DESC
                    LIMIT 8;
                    """,
                    (dato["traduccion_id"],),
                )
                relacionadas = cur.fetchall()

    return render_template("noticia.html", dato=dato, tags=tags, relacionadas=relacionadas)

