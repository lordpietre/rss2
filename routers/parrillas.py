"""
Router para gestionar parrill

as de videos de noticias.
"""
from flask import Blueprint, render_template, request, jsonify, redirect, url_for, flash
from db import get_conn
from psycopg2 import extras
from datetime import datetime, timedelta
import logging

logger = logging.getLogger(__name__)

parrillas_bp = Blueprint("parrillas", __name__, url_prefix="/parrillas")


@parrillas_bp.route("/")
def index():
    """Dashboard principal de parrillas."""
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Obtener todas las parrillas
            cur.execute("""
                SELECT 
                    p.*,
                    pa.nombre as pais_nombre,
                    c.nombre as categoria_nombre,
                    (SELECT COUNT(*) FROM video_generados WHERE parrilla_id = p.id) as total_videos
                FROM video_parrillas p
                LEFT JOIN paises pa ON pa.id = p.pais_id
                LEFT JOIN categorias c ON c.id = p.categoria_id
                ORDER BY p.created_at DESC
            """)
            parrillas = cur.fetchall()
            
    return render_template("parrillas/index.html", parrillas=parrillas)


@parrillas_bp.route("/nueva", methods=["GET", "POST"])
def nueva():
    """Crear una nueva parrilla."""
    if request.method == "GET":
        # Cargar datos para el formulario
        with get_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                cur.execute("SELECT id, nombre FROM paises ORDER BY nombre")
                paises = cur.fetchall()
                
                cur.execute("SELECT id, nombre FROM categorias ORDER BY nombre")
                categorias = cur.fetchall()
                
        return render_template("parrillas/form.html", 
                             paises=paises, 
                             categorias=categorias)
    
    # POST: Crear parrilla
    try:
        data = request.form
        
        with get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("""
                    INSERT INTO video_parrillas (
                        nombre, descripcion, tipo_filtro,
                        pais_id, categoria_id, entidad_nombre, entidad_tipo,
                        max_noticias, duracion_maxima, idioma_voz,
                        template, include_images, include_subtitles,
                        frecuencia, activo
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                    ) RETURNING id
                """, (
                    data.get('nombre'),
                    data.get('descripcion'),
                    data.get('tipo_filtro'),
                    data.get('pais_id') or None,
                    data.get('categoria_id') or None,
                    data.get('entidad_nombre') or None,
                    data.get('entidad_tipo') or None,
                    int(data.get('max_noticias', 5)),
                    int(data.get('duracion_maxima', 180)),
                    data.get('idioma_voz', 'es'),
                    data.get('template', 'standard'),
                    data.get('include_images') == 'on',
                    data.get('include_subtitles') == 'on',
                    data.get('frecuencia', 'manual'),
                    data.get('activo') == 'on'
                ))
                parrilla_id = cur.fetchone()[0]
                conn.commit()
                
        flash(f"Parrilla '{data.get('nombre')}' creada exitosamente", "success")
        return redirect(url_for('parrillas.ver', id=parrilla_id))
        
    except Exception as e:
        logger.error(f"Error creating parrilla: {e}", exc_info=True)
        flash(f"Error al crear parrilla: {str(e)}", "error")
        return redirect(url_for('parrillas.nueva'))


@parrillas_bp.route("/<int:id>")
def ver(id):
    """Ver detalles de una parrilla específica."""
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Obtener parrilla
            cur.execute("""
                SELECT 
                    p.*,
                    pa.nombre as pais_nombre,
                    c.nombre as categoria_nombre
                FROM video_parrillas p
                LEFT JOIN paises pa ON pa.id = p.pais_id
                LEFT JOIN categorias c ON c.id = p.categoria_id
                WHERE p.id = %s
            """, (id,))
            parrilla = cur.fetchone()
            
            if not parrilla:
                flash("Parrilla no encontrada", "error")
                return redirect(url_for('parrillas.index'))
            
            # Obtener videos generados
            cur.execute("""
                SELECT * FROM video_generados
                WHERE parrilla_id = %s
                ORDER BY fecha_generacion DESC
                LIMIT 50
            """, (id,))
            videos = cur.fetchall()
            
    return render_template("parrillas/detail.html", parrilla=parrilla, videos=videos)


@parrillas_bp.route("/api/<int:id>/preview")
def preview_noticias(id):
    """Preview de noticias que se incluirían en el siguiente video."""
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Obtener configuración de parrilla
            cur.execute("SELECT * FROM video_parrillas WHERE id = %s", (id,))
            parrilla = cur.fetchone()
            
            if not parrilla:
                return jsonify({"error": "Parrilla no encontrada"}), 404
            
            # Construir query según filtros
            where_clauses = []
            params = []
            
            if parrilla['pais_id']:
                where_clauses.append("n.pais_id = %s")
                params.append(parrilla['pais_id'])
                
            if parrilla['categoria_id']:
                where_clauses.append("n.categoria_id = %s")
                params.append(parrilla['categoria_id'])
                
            if parrilla['entidad_nombre']:
                # Filtrar por entidad
                where_clauses.append("""
                    EXISTS (
                        SELECT 1 FROM tags_noticia tn
                        JOIN tags t ON t.id = tn.tag_id
                        WHERE tn.traduccion_id = tr.id
                        AND t.tipo = %s
                        AND t.valor ILIKE %s
                    )
                """)
                params.append(parrilla['entidad_tipo'])
                params.append(f"%{parrilla['entidad_nombre']}%")
            
            # Solo noticias de hoy o ayer
            where_clauses.append("n.fecha >= NOW() - INTERVAL '1 day'")
            
            where_sql = " AND ".join(where_clauses) if where_clauses else "1=1"
            
            # Obtener noticias
            cur.execute(f"""
                SELECT 
                    n.id,
                    n.titulo,
                    n.imagen_url,
                    n.fecha,
                    tr.titulo_trad,
                    tr.resumen_trad,
                    LENGTH(tr.resumen_trad) as longitud_texto
                FROM noticias n
                LEFT JOIN traducciones tr ON tr.noticia_id = n.id AND tr.lang_to = %s AND tr.status = 'done'
                WHERE {where_sql}
                AND tr.id IS NOT NULL
                ORDER BY n.fecha DESC
                LIMIT %s
            """, [parrilla['idioma_voz']] + params + [parrilla['max_noticias']])
            
            noticias = cur.fetchall()
            
    return jsonify({
        "noticias": [dict(n) for n in noticias],
        "total": len(noticias),
        "config": {
            "max_noticias": parrilla['max_noticias'],
            "duracion_maxima": parrilla['duracion_maxima']
        }
    })


@parrillas_bp.route("/api/<int:id>/generar", methods=["POST"])
def generar_video(id):
    """Iniciar generación de video para una parrilla."""
    try:
        with get_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                # Verificar que la parrilla existe
                cur.execute("SELECT * FROM video_parrillas WHERE id = %s", (id,))
                parrilla = cur.fetchone()
                
                if not parrilla:
                    return jsonify({"error": "Parrilla no encontrada"}), 404
                
                # Crear registro de video
                cur.execute("""
                    INSERT INTO video_generados (
                        parrilla_id, titulo, descripcion, status
                    ) VALUES (
                        %s, %s, %s, 'pending'
                    ) RETURNING id
                """, (
                    id,
                    f"{parrilla['nombre']} - {datetime.now().strftime('%Y-%m-%d %H:%M')}",
                    f"Video generado automáticamente para {parrilla['nombre']}"
                ))
                video_id = cur.fetchone()[0]
                
                # Actualizar fecha de última generación
                cur.execute("""
                    UPDATE video_parrillas
                    SET ultima_generacion = NOW()
                    WHERE id = %s
                """, (id,))
                
                conn.commit()
                
        # Lanzar el proceso de generación en segundo plano
        import subprocess
        import sys
        
        # Ejecutamos el script generador pasando el ID de la parrilla
        # Usamos Popen para no bloquear la respuesta HTTP (fire and forget)
        cmd = [sys.executable, "generar_videos_noticias.py", str(id)]
        subprocess.Popen(cmd, cwd="/app")
        
        return jsonify({
            "success": True,
            "video_id": video_id,
            "message": "Generación de video iniciada en segundo plano"
        })
        
    except Exception as e:
        logger.error(f"Error queuing video: {e}", exc_info=True)
        return jsonify({"error": str(e)}), 500


@parrillas_bp.route("/api/<int:id>", methods=["DELETE"])
def eliminar(id):
    """Eliminar una parrilla."""
    try:
        with get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM video_parrillas WHERE id = %s", (id,))
                conn.commit()
                
        return jsonify({"success": True})
    except Exception as e:
        logger.error(f"Error deleting parrilla: {e}", exc_info=True)
        return jsonify({"error": str(e)}), 500


@parrillas_bp.route("/api/<int:id>/toggle", methods=["POST"])
def toggle_activo(id):
    """Activar/desactivar una parrilla."""
    try:
        with get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("""
                    UPDATE video_parrillas
                    SET activo = NOT activo
                    WHERE id = %s
                    RETURNING activo
                """, (id,))
                nuevo_estado = cur.fetchone()[0]
                conn.commit()
                
        return jsonify({"success": True, "activo": nuevo_estado})
    except Exception as e:
        logger.error(f"Error toggling parrilla: {e}", exc_info=True)
        return jsonify({"error": str(e)}), 500


@parrillas_bp.route("/files/<int:video_id>/<filename>")
def serve_file(video_id, filename):
    """Servir archivos generados (audio, script, srt)."""
    from flask import send_from_directory
    import os
    
    # Directorio base de videos
    base_dir = "/app/data/videos"
    video_dir = os.path.join(base_dir, str(video_id))
    
    # Validar que sea un archivo permitido para evitar Path Traversal
    allowed_files = ['audio.wav', 'script.txt', 'subtitles.srt', 'generation.log']
    if filename not in allowed_files:
        logger.warning(f"File download attempt blocked: {filename}")
        return "File not allowed", 403
        
    full_path = os.path.join(video_dir, filename)
    if not os.path.exists(full_path):
        logger.error(f"File not found: {full_path}")
        return "File not found", 404
        
    try:
        return send_from_directory(video_dir, filename)
    except Exception as e:
        logger.error(f"Error serving file {full_path}: {e}")
        return f"Error serving file: {e}", 500
