from flask import Blueprint, render_template, request, redirect, flash, url_for, jsonify
from db import get_conn
from psycopg2 import extras
from models.categorias import get_categorias
from models.paises import get_paises
from utils.feed_discovery import discover_feeds, validate_feed, get_feed_metadata

# Blueprint correcto
feeds_bp = Blueprint("feeds", __name__, url_prefix="/feeds")


@feeds_bp.route("/")
def list_feeds():
    """Listado con filtros"""
    page = max(int(request.args.get("page", 1)), 1)
    per_page = 50
    offset = (page - 1) * per_page

    pais_id = request.args.get("pais_id")
    categoria_id = request.args.get("categoria_id")
    estado = request.args.get("estado") or ""

    where = []
    params = []

    if pais_id:
        where.append("f.pais_id = %s")
        params.append(int(pais_id))

    if categoria_id:
        where.append("f.categoria_id = %s")
        params.append(int(categoria_id))

    if estado == "activos":
        where.append("f.activo = TRUE")
    elif estado == "inactivos":
        where.append("f.activo = FALSE")
    elif estado == "errores":
        where.append("COALESCE(f.fallos, 0) > 0")

    where_sql = "WHERE " + " AND ".join(where) if where else ""

    with get_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:

        # Total
        # Total
        cur.execute(f"SELECT COUNT(*) FROM feeds f {where_sql}", params)
        total_feeds = cur.fetchone()[0]
        
        # Caídos (Inactivos o con max fallos logic check, usually inactive is enough if logic works)
        # Using the same filter context to see how many of THESE are fallen
        # Caídos (Inactivos o con max fallos logic check)
        # Using the same filter context to see how many of THESE are fallen
        
        caidos_condition = "(f.activo = FALSE OR f.fallos >= 5)"
        
        if where_sql:
            # where_sql ya incluye "WHERE ..."
            caidos_sql = f"SELECT COUNT(*) FROM feeds f {where_sql} AND {caidos_condition}"
        else:
            caidos_sql = f"SELECT COUNT(*) FROM feeds f WHERE {caidos_condition}"
            
        cur.execute(caidos_sql, params)
        feeds_caidos = cur.fetchone()[0]

        total_pages = (total_feeds // per_page) + (1 if total_feeds % per_page else 0)

        # Lista paginada
        cur.execute(
            f"""
            SELECT
                f.id, f.nombre, f.descripcion, f.url,
                f.activo, f.fallos, f.last_error,
                c.nombre AS categoria,
                p.nombre AS pais,
                (SELECT COUNT(*) FROM noticias n WHERE n.fuente_nombre = f.nombre) as noticias_count
            FROM feeds f
            LEFT JOIN categorias c ON c.id = f.categoria_id
            LEFT JOIN paises p ON p.id = f.pais_id
            {where_sql}
            ORDER BY p.nombre NULLS LAST, f.activo DESC, f.fallos ASC, c.nombre NULLS LAST, f.nombre
            LIMIT %s OFFSET %s
            """,
            params + [per_page, offset],
        )
        feeds = cur.fetchall()

        # Selects
        cur.execute("SELECT id, nombre FROM categorias ORDER BY nombre;")
        categorias = cur.fetchall()

        cur.execute("SELECT id, nombre FROM paises ORDER BY nombre;")
        paises = cur.fetchall()

    if request.headers.get("X-Requested-With") == "XMLHttpRequest":
        return render_template(
            "_feeds_table.html",
            feeds=feeds,
            total_feeds=total_feeds,
            feeds_caidos=feeds_caidos,
            total_pages=total_pages,
            page=page,
            filtro_pais_id=pais_id,
            filtro_categoria_id=categoria_id,
            filtro_estado=estado,
        )

    return render_template(
        "feeds_list.html",
        feeds=feeds,
        total_feeds=total_feeds,
        feeds_caidos=feeds_caidos,
        total_pages=total_pages,
        page=page,
        categorias=categorias,
        paises=paises,
        filtro_pais_id=pais_id,
        filtro_categoria_id=categoria_id,
        filtro_estado=estado,
    )


@feeds_bp.route("/add", methods=["GET", "POST"])
def add_feed():
    """Añadir feed"""
    with get_conn() as conn:
        categorias = get_categorias(conn)
        paises = get_paises(conn)

        if request.method == "POST":
            nombre = request.form.get("nombre")
            descripcion = request.form.get("descripcion") or None
            url = request.form.get("url")
            categoria_id = request.form.get("categoria_id")
            pais_id = request.form.get("pais_id")
            idioma = (request.form.get("idioma") or "").strip().lower()[:2] or None

            try:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma)
                        VALUES (%s, %s, %s, %s, %s, %s)
                        """,
                        (
                            nombre,
                            descripcion,
                            url,
                            int(categoria_id) if categoria_id else None,
                            int(pais_id) if pais_id else None,
                            idioma,
                        ),
                    )
                conn.commit()
                flash("Feed añadido correctamente.", "success")
                return redirect(url_for("feeds.list_feeds"))

            except Exception as e:
                flash(f"Error al añadir feed: {e}", "error")

    return render_template("add_feed.html", categorias=categorias, paises=paises)


@feeds_bp.route("/<int:feed_id>/edit", methods=["GET", "POST"])
def edit_feed(feed_id):
    """Editar feed"""
    with get_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:

        cur.execute("SELECT * FROM feeds WHERE id = %s;", (feed_id,))
        feed = cur.fetchone()

        if not feed:
            flash("Feed no encontrado.", "error")
            return redirect(url_for("feeds.list_feeds"))

        categorias = get_categorias(conn)
        paises = get_paises(conn)

        if request.method == "POST":
            nombre = request.form.get("nombre")
            descripcion = request.form.get("descripcion") or None
            url = request.form.get("url")
            categoria_id = request.form.get("categoria_id")
            pais_id = request.form.get("pais_id")
            idioma = (request.form.get("idioma") or "").strip().lower()[:2] or None
            activo = bool(request.form.get("activo"))

            try:
                cur.execute(
                    """
                    UPDATE feeds
                    SET nombre=%s, descripcion=%s, url=%s,
                        categoria_id=%s, pais_id=%s, idioma=%s, activo=%s
                    WHERE id=%s;
                    """,
                    (
                        nombre,
                        descripcion,
                        url,
                        int(categoria_id) if categoria_id else None,
                        int(pais_id) if pais_id else None,
                        idioma,
                        activo,
                        feed_id,
                    ),
                )
                conn.commit()

                flash("Feed actualizado.", "success")
                return redirect(url_for("feeds.list_feeds"))

            except Exception as e:
                flash(f"Error al actualizar: {e}", "error")

    return render_template("edit_feed.html", feed=feed, categorias=categorias, paises=paises)


@feeds_bp.route("/<int:feed_id>/delete")
def delete_feed(feed_id):
    """Eliminar feed"""
    with get_conn() as conn, conn.cursor() as cur:
        try:
            cur.execute("DELETE FROM feeds WHERE id=%s;", (feed_id,))
            conn.commit()
            flash("Feed eliminado.", "success")
        except Exception as e:
            flash(f"No se pudo eliminar: {e}", "error")

    return redirect(url_for("feeds.list_feeds"))


@feeds_bp.route("/<int:feed_id>/reactivar")
def reactivar_feed(feed_id):
    """Reactivar feed KO"""
    with get_conn() as conn, conn.cursor() as cur:
        try:
            cur.execute(
                "UPDATE feeds SET activo=TRUE, fallos=0 WHERE id=%s;",
                (feed_id,),
            )
            conn.commit()
            flash("Feed reactivado.", "success")
        except Exception as e:
            flash(f"No se pudo reactivar: {e}", "error")

    return redirect(url_for("feeds.list_feeds"))


@feeds_bp.route("/discover", methods=["GET", "POST"])
def discover_feed():
    """Descubrir feeds RSS desde una URL"""
    discovered_feeds = []
    source_url = ""
    
    with get_conn() as conn:
        categorias = get_categorias(conn)
        paises = get_paises(conn)
        
        if request.method == "POST":
            source_url = request.form.get("source_url", "").strip()
            
            if not source_url:
                flash("Por favor, ingresa una URL válida.", "error")
            else:
                try:
                    # Discover feeds from the URL
                    discovered_feeds = discover_feeds(source_url, timeout=15)
                    
                    if not discovered_feeds:
                        flash(f"No se encontraron feeds RSS en la URL: {source_url}", "warning")
                    else:
                        # Check which feeds already exist in DB
                        found_urls = [f['url'] for f in discovered_feeds]
                        existing_urls = set()
                        try:
                            with conn.cursor() as cur:
                                cur.execute("SELECT url FROM feeds WHERE url = ANY(%s)", (found_urls,))
                                rows = cur.fetchall()
                                existing_urls = {r[0] for r in rows}
                        except Exception as db_e:
                             # Fallback if DB fails, though unlikely
                             print(f"Error checking existing feeds: {db_e}")

                        for feed in discovered_feeds:
                            feed['exists'] = feed['url'] in existing_urls

                        new_count = len(discovered_feeds) - len(existing_urls)
                        flash(f"Feeds disponibles: {new_count} de {len(discovered_feeds)} encontrados.", "success")
                        
                except Exception as e:
                    flash(f"Error al descubrir feeds: {e}", "error")
    
    return render_template(
        "discover_feeds.html",
        discovered_feeds=discovered_feeds,
        source_url=source_url,
        categorias=categorias,
        paises=paises
    )


@feeds_bp.route("/discover_and_add", methods=["POST"])
def discover_and_add():
    """Añadir múltiples feeds descubiertos"""
    selected_feeds = request.form.getlist("selected_feeds")
    categoria_id = request.form.get("categoria_id")
    pais_id = request.form.get("pais_id")
    idioma = (request.form.get("idioma") or "").strip().lower()[:2] or None
    
    if not selected_feeds:
        flash("No se seleccionó ningún feed.", "warning")
        return redirect(url_for("feeds.discover_feed"))
    
    added_count = 0
    errors = []
    
    with get_conn() as conn:
        for feed_url in selected_feeds:
            try:
                # Get individual settings for this feed
                # The form uses the feed URL as part of the field name
                item_cat_id = request.form.get(f"cat_{feed_url}")
                item_country_id = request.form.get(f"country_{feed_url}")
                item_lang = request.form.get(f"lang_{feed_url}")
                
                # Get feed metadata
                metadata = get_feed_metadata(feed_url, timeout=10)
                
                if not metadata:
                    errors.append(f"No se pudo obtener metadata del feed: {feed_url}")
                    continue
                
                # Use context title from discovery if available, otherwise use metadata title
                context_title = request.form.get(f"context_{feed_url}")
                nombre = context_title if context_title else metadata.get('title', 'Feed sin título')
                
                descripcion = metadata.get('description', '')
                
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma)
                        VALUES (%s, %s, %s, %s, %s, %s)
                        ON CONFLICT (url) DO NOTHING
                        """,
                        (
                            nombre,
                            descripcion[:500] if descripcion else None,
                            feed_url,
                            int(item_cat_id) if item_cat_id else None,
                            int(item_country_id) if item_country_id else None,
                            (item_lang or "").strip().lower()[:2] or None,
                        ),
                    )
                    if cur.rowcount > 0:
                        added_count += 1
                        
                conn.commit()
                
            except Exception as e:
                errors.append(f"Error al añadir {feed_url}: {e}")
    
    is_ajax = request.headers.get("X-Requested-With") == "XMLHttpRequest"

    if added_count > 0:
        msg = f"Se añadieron {added_count} feeds correctamente."
        if not is_ajax:
            flash(msg, "success")
    else:
        msg = "No se añadieron feeds nuevos."
        if not is_ajax:
             # Only flash warning if not ajax, or handle differently
             if not errors:
                 flash(msg, "warning")
    
    if errors:
        for error in errors[:5]:  # Mostrar solo los primeros 5 errores
            if not is_ajax:
                flash(error, "error")

    if is_ajax:
        return jsonify({
            "success": added_count > 0,
            "added_count": added_count,
            "message": msg,
            "errors": errors
        })
    
    return redirect(url_for("feeds.list_feeds"))


@feeds_bp.route("/api/validate", methods=["POST"])
def api_validate_feed():
    """API endpoint para validar una URL de feed"""
    data = request.get_json()
    feed_url = data.get("url", "").strip()
    
    if not feed_url:
        return jsonify({"error": "URL no proporcionada"}), 400
    
    try:
        feed_info = validate_feed(feed_url, timeout=10)
        
        if not feed_info:
            return jsonify({"error": "No se pudo validar el feed"}), 400
        
        return jsonify(feed_info), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@feeds_bp.route("/api/discover", methods=["POST"])
def api_discover_feeds():
    """API endpoint para descubrir feeds desde una URL"""
    data = request.get_json()
    source_url = data.get("url", "").strip()
    
    if not source_url:
        return jsonify({"error": "URL no proporcionada"}), 400
    
    try:
        discovered = discover_feeds(source_url, timeout=15)
        return jsonify({"feeds": discovered, "count": len(discovered)}), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

