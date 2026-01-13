from flask import Blueprint, send_file, render_template, request, flash, redirect, url_for
import csv
import io
from psycopg2 import extras
from db import get_conn

backup_bp = Blueprint("backup", __name__)


# ============================================================
#   EXPORTAR FEEDS → CSV (OK)
# ============================================================

@backup_bp.route("/backup_feeds")
def backup_feeds():
    with get_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute("""
            SELECT f.id, f.nombre, f.descripcion, f.url,
                   f.categoria_id, c.nombre AS categoria,
                   f.pais_id, p.nombre AS pais,
                   f.idioma, f.activo, f.fallos
            FROM feeds f
            LEFT JOIN categorias c ON c.id=f.categoria_id
            LEFT JOIN paises p ON p.id=f.pais_id
            ORDER BY f.id;
        """)
        rows = cur.fetchall()

    output = io.StringIO()
    writer = csv.writer(output)

    writer.writerow([
        "id", "nombre", "descripcion", "url",
        "categoria_id", "categoria",
        "pais_id", "pais",
        "idioma", "activo", "fallos"
    ])

    for r in rows:
        writer.writerow([
            r["id"],
            r["nombre"],
            r["descripcion"] or "",
            r["url"],
            r["categoria_id"] or "",
            r["categoria"] or "",
            r["pais_id"] or "",
            r["pais"] or "",
            r["idioma"] or "",
            r["activo"],
            r["fallos"],
        ])

    output.seek(0)
    return send_file(
        io.BytesIO(output.getvalue().encode("utf-8")),
        mimetype="text/csv",
        as_attachment=True,
        download_name="feeds_backup.csv",
    )


# ============================================================
#   EXPORTAR FEEDS FILTRADOS → CSV
# ============================================================

@backup_bp.route("/export_feeds_filtered")
def export_feeds_filtered():
    """Exportar feeds con filtros opcionales (país, categoría, estado)."""
    pais_id = request.args.get("pais_id")
    categoria_id = request.args.get("categoria_id")
    estado = request.args.get("estado") or ""

    # Construir filtros WHERE (misma lógica que list_feeds)
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

    # Query SQL con filtros
    with get_conn() as conn, conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute(f"""
            SELECT f.id, f.nombre, f.descripcion, f.url,
                   f.categoria_id, c.nombre AS categoria,
                   f.pais_id, p.nombre AS pais,
                   f.idioma, f.activo, f.fallos
            FROM feeds f
            LEFT JOIN categorias c ON c.id=f.categoria_id
            LEFT JOIN paises p ON p.id=f.pais_id
            {where_sql}
            ORDER BY p.nombre NULLS LAST, c.nombre NULLS LAST, f.nombre;
        """, params)
        rows = cur.fetchall()

        # Obtener nombres para el archivo
        pais_nombre = None
        categoria_nombre = None
        
        if pais_id:
            cur.execute("SELECT nombre FROM paises WHERE id = %s", (int(pais_id),))
            result = cur.fetchone()
            if result:
                pais_nombre = result["nombre"]
        
        if categoria_id:
            cur.execute("SELECT nombre FROM categorias WHERE id = %s", (int(categoria_id),))
            result = cur.fetchone()
            if result:
                categoria_nombre = result["nombre"]

    # Generar CSV
    output = io.StringIO()
    writer = csv.writer(output)

    writer.writerow([
        "id", "nombre", "descripcion", "url",
        "categoria_id", "categoria",
        "pais_id", "pais",
        "idioma", "activo", "fallos"
    ])

    for r in rows:
        writer.writerow([
            r["id"],
            r["nombre"],
            r["descripcion"] or "",
            r["url"],
            r["categoria_id"] or "",
            r["categoria"] or "",
            r["pais_id"] or "",
            r["pais"] or "",
            r["idioma"] or "",
            r["activo"],
            r["fallos"],
        ])

    # Generar nombre de archivo dinámico
    filename_parts = ["feeds"]
    
    if pais_nombre:
        # Limpiar nombre de país para usar en archivo
        clean_pais = pais_nombre.lower().replace(" ", "_").replace("/", "_")
        filename_parts.append(clean_pais)
    
    if categoria_nombre:
        clean_cat = categoria_nombre.lower().replace(" ", "_").replace("/", "_")
        filename_parts.append(clean_cat)
    
    if estado:
        filename_parts.append(estado)
    
    filename = "_".join(filename_parts) + ".csv"

    output.seek(0)
    return send_file(
        io.BytesIO(output.getvalue().encode("utf-8")),
        mimetype="text/csv",
        as_attachment=True,
        download_name=filename,
    )


# ============================================================
#   RESTAURAR FEEDS → CSV  (VERSIÓN PROFESIONAL)
# ============================================================

@backup_bp.route("/restore_feeds", methods=["GET", "POST"])
def restore_feeds():
    if request.method == "GET":
        return render_template("restore_feeds.html")

    file = request.files.get("file")
    if not file:
        flash("Debes seleccionar un archivo CSV.", "error")
        return redirect(url_for("backup.restore_feeds"))

    # 1) Leer CSV
    try:
        raw = file.read().decode("utf-8-sig").replace("\ufeff", "")
        reader = csv.DictReader(io.StringIO(raw))
    except Exception as e:
        flash(f"Error al procesar CSV: {e}", "error")
        return redirect(url_for("backup.restore_feeds"))

    expected_fields = [
        "id", "nombre", "descripcion", "url",
        "categoria_id", "categoria",
        "pais_id", "pais",
        "idioma", "activo", "fallos"
    ]

    if reader.fieldnames != expected_fields:
        flash("El CSV no tiene el encabezado correcto.", "error")
        return redirect(url_for("backup.restore_feeds"))

    # Contadores
    imported = 0
    skipped = 0
    failed = 0

    with get_conn() as conn:
        with conn.cursor() as cur:

                # Vaciar tabla ELIMINADO para no borrar feeds existentes
            # cur.execute("TRUNCATE feeds RESTART IDENTITY CASCADE;")

            for row in reader:
                # Limpieza general
                row = {k: (v.strip().rstrip("ç") if isinstance(v, str) else v) for k, v in row.items()}

                # Validaciones mínimas
                if not row["url"] or not row["nombre"]:
                    skipped += 1
                    continue

                try:
                    # Creating a savepoint to isolate this row's transaction
                    cur.execute("SAVEPOINT row_savepoint")

                    # Normalizar valores
                    categoria_id = int(row["categoria_id"]) if row["categoria_id"] else None
                    pais_id = int(row["pais_id"]) if row["pais_id"] else None

                    idioma = (row["idioma"] or "").lower().strip()
                    idioma = idioma[:2] if idioma else None

                    activo = str(row["activo"]).lower() in ("true", "1", "t", "yes", "y")
                    fallos = int(row["fallos"] or 0)

                    # Buscar si ya existe un feed con esta URL
                    cur.execute("SELECT id FROM feeds WHERE url = %s", (row["url"],))
                    existing_feed = cur.fetchone()

                    if existing_feed:
                        # URL ya existe -> ACTUALIZAR el feed existente
                        cur.execute("""
                            UPDATE feeds SET
                                nombre=%s,
                                descripcion=%s,
                                categoria_id=%s,
                                pais_id=%s,
                                idioma=%s,
                                activo=%s,
                                fallos=%s
                            WHERE id=%s
                        """, (
                            row["nombre"],
                            row["descripcion"] or None,
                            categoria_id,
                            pais_id,
                            idioma,
                            activo,
                            fallos,
                            existing_feed[0]
                        ))
                    else:
                        # URL no existe -> INSERTAR NUEVO feed (ignorar ID del CSV, usar auto-increment)
                        cur.execute("""
                            INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma, activo, fallos)
                            VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
                        """, (
                            row["nombre"],
                            row["descripcion"] or None,
                            row["url"],
                            categoria_id,
                            pais_id,
                            idioma,
                            activo,
                            fallos
                        ))
                    
                    cur.execute("RELEASE SAVEPOINT row_savepoint")
                    imported += 1

                except Exception as e:
                    # If any error happens, rollback to the savepoint so the main transaction isn't aborted
                    cur.execute("ROLLBACK TO SAVEPOINT row_savepoint")
                    failed += 1
                    continue

            # No need to reset sequence - auto-increment handles it
            conn.commit()

    flash(
        f"Restauración completada. "
        f"Importados: {imported} | Saltados: {skipped} | Fallidos: {failed}",
        "success"
    )

    return redirect(url_for("feeds.list_feeds"))


# ============================================================
#   EXPORTAR METADATOS (PAISES / CATEGORIAS)
# ============================================================

@backup_bp.route("/export_paises")
def export_paises():
    """Exportar listado de países a CSV."""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute("SELECT id, nombre FROM paises ORDER BY id;")
        rows = cur.fetchall()

    output = io.StringIO()
    writer = csv.writer(output)
    writer.writerow(["id", "nombre"])
    for r in rows:
        writer.writerow([r[0], r[1]])

    output.seek(0)
    return send_file(
        io.BytesIO(output.getvalue().encode("utf-8")),
        mimetype="text/csv",
        as_attachment=True,
        download_name="paises.csv",
    )


@backup_bp.route("/export_categorias")
def export_categorias():
    """Exportar listado de categorías a CSV."""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute("SELECT id, nombre FROM categorias ORDER BY id;")
        rows = cur.fetchall()

    output = io.StringIO()
    writer = csv.writer(output)
    writer.writerow(["id", "nombre"])
    for r in rows:
        writer.writerow([r[0], r[1]])

    output.seek(0)
    return send_file(
        io.BytesIO(output.getvalue().encode("utf-8")),
        mimetype="text/csv",
        as_attachment=True,
        download_name="categorias.csv",
    )
