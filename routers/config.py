from flask import Blueprint, render_template, request, redirect, url_for, flash, Response, stream_with_context
from datetime import datetime
import json
import zipfile
import io
from db import get_conn
from psycopg2 import extras

config_bp = Blueprint("config", __name__, url_prefix="/config")

@config_bp.route("/")
def config_home():
    return render_template("config.html")

import tempfile
import os
import shutil
import threading
import uuid
import time
from flask import send_file, jsonify
from cache import cache_set, cache_get

# Global dictionary to store temporary file paths (optional, but Redis is safer for clustered env)
# Since we are in a single-server Docker setup, a global dict is fine for paths if we don't restart.
# But for absolute safety, we'll store paths in Redis too.
BACKUP_TASKS = {}

@config_bp.route("/backup/start")
def backup_start():
    task_id = str(uuid.uuid4())
    cache_set(f"backup_status:{task_id}", {"progress": 0, "total": 0, "status": "initializing"})
    
    # Start thread
    thread = threading.Thread(target=_backup_worker, args=(task_id,))
    thread.daemon = True
    thread.start()
    
    return jsonify({"task_id": task_id})

@config_bp.route("/backup/status/<task_id>")
def backup_status(task_id):
    status = cache_get(f"backup_status:{task_id}")
    if not status:
        return jsonify({"error": "Task not found"}), 404
    return jsonify(status)

@config_bp.route("/backup/download/<task_id>")
def backup_download(task_id):
    status = cache_get(f"backup_status:{task_id}")
    if not status or status.get("status") != "completed":
        return "Archivo no listo o expirado", 404
    
    file_path = status.get("file_path")
    if not file_path or not os.path.exists(file_path):
        return "Archivo no encontrado", 404
    
    filename = f"backup_noticias_{datetime.now().strftime('%Y%m%d_%H%M%S')}.zip"
    return send_file(file_path, as_attachment=True, download_name=filename)

import io

def _backup_worker(task_id):
    """Background thread to generate the backup ZIP with direct streaming."""
    print(f"[BACKUP {task_id}] Inicia proceso...")
    try:
        tmp_dir = tempfile.mkdtemp()
        timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
        zip_path = os.path.join(tmp_dir, f"backup_{timestamp}.zip")
        
        from db import get_read_conn # Use replica for large reads
        
        with get_read_conn() as conn:
            # 1. Count totals for progress
            print(f"[BACKUP {task_id}] Contando registros...")
            with conn.cursor() as cur:
                cur.execute("SELECT count(*) FROM noticias")
                total_n = cur.fetchone()[0]
                cur.execute("SELECT count(*) FROM traducciones WHERE status = 'done'")
                total_t = cur.fetchone()[0]
            
            total_total = total_n + total_t
            print(f"[BACKUP {task_id}] Total registros: {total_total}")
            cache_set(f"backup_status:{task_id}", {"progress": 0, "total": total_total, "status": "processing"})
            
            processed = 0
            with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
                # --- NOTICIAS ---
                print(f"[BACKUP {task_id}] Exportando noticias...")
                with zf.open("noticias.jsonl", "w") as bf:
                    # Wrap binary file for text writing
                    with io.TextIOWrapper(bf, encoding='utf-8') as f:
                        with conn.cursor(name=f'bak_n_{task_id}', cursor_factory=extras.DictCursor) as cur:
                            cur.itersize = 2000
                            cur.execute("SELECT id, titulo, resumen, url, fecha, imagen_url, fuente_nombre, categoria_id, pais_id FROM noticias")
                            for row in cur:
                                item = dict(row)
                                if item.get("fecha"): item["fecha"] = item["fecha"].isoformat()
                                f.write(json.dumps(item, ensure_ascii=False) + "\n")
                                processed += 1
                                if processed % 2000 == 0:
                                    cache_set(f"backup_status:{task_id}", {"progress": processed, "total": total_total, "status": "processing"})
                
                # --- TRADUCCIONES ---
                print(f"[BACKUP {task_id}] Exportando traducciones...")
                with zf.open("traducciones.jsonl", "w") as bf:
                    with io.TextIOWrapper(bf, encoding='utf-8') as f:
                        with conn.cursor(name=f'bak_t_{task_id}', cursor_factory=extras.DictCursor) as cur:
                            cur.itersize = 2000
                            cur.execute("SELECT id, noticia_id, lang_from, lang_to, titulo_trad, resumen_trad, status, created_at FROM traducciones WHERE status = 'done'")
                            for row in cur:
                                item = dict(row)
                                if item.get("created_at"): item["created_at"] = item["created_at"].isoformat()
                                f.write(json.dumps(item, ensure_ascii=False) + "\n")
                                processed += 1
                                if processed % 2000 == 0:
                                    cache_set(f"backup_status:{task_id}", {"progress": processed, "total": total_total, "status": "processing"})

        print(f"[BACKUP {task_id}] Finalizado con éxito: {zip_path}")
        cache_set(f"backup_status:{task_id}", {
            "progress": total_total, 
            "total": total_total, 
            "status": "completed", 
            "file_path": zip_path
        }, ttl_seconds=3600)
        
    except Exception as e:
        import traceback
        error_msg = traceback.format_exc()
        print(f"[BACKUP {task_id}] ERROR: {error_msg}")
        cache_set(f"backup_status:{task_id}", {"status": "error", "error": str(e)})

@config_bp.route("/restore/noticias", methods=["GET", "POST"])
def restore_noticias():
    # Keep current restore logic but maybe add progress too? 
    # For now let's focus on fix the client's immediate backup download issue.
    if request.method == "GET":
        return render_template("config_restore.html")
    
    file = request.files.get("file")
    if not file:
        flash("Debes seleccionar un archivo ZIP.", "error")
        return redirect(url_for("config.restore_noticias"))
    
    if not file.filename.endswith(".zip"):
        flash("El formato debe ser .zip", "error")
        return redirect(url_for("config.restore_noticias"))

    imported_n = 0
    imported_t = 0
    
    tmp_zip = tempfile.NamedTemporaryFile(delete=False, suffix=".zip")
    file.save(tmp_zip.name)
    tmp_zip.close()

    try:
        with zipfile.ZipFile(tmp_zip.name, "r") as zf:
            if "noticias.jsonl" in zf.namelist():
                with zf.open("noticias.jsonl") as f:
                    chunk = []
                    for line in f:
                        chunk.append(json.loads(line.decode("utf-8")))
                        if len(chunk) >= 500:
                            _import_noticias_chunk(chunk)
                            imported_n += len(chunk)
                            chunk = []
                    if chunk:
                        _import_noticias_chunk(chunk)
                        imported_n += len(chunk)

            if "traducciones.jsonl" in zf.namelist():
                with zf.open("traducciones.jsonl") as f:
                    chunk = []
                    for line in f:
                        chunk.append(json.loads(line.decode("utf-8")))
                        if len(chunk) >= 500:
                            _import_traducciones_chunk(chunk)
                            imported_t += len(chunk)
                            chunk = []
                    if chunk:
                        _import_traducciones_chunk(chunk)
                        imported_t += len(chunk)
    finally:
        if os.path.exists(tmp_zip.name):
            os.remove(tmp_zip.name)
    
    flash(f"Restauración completada: {imported_n} noticias, {imported_t} traducciones.", "success")
    return redirect(url_for("config.config_home"))

def _import_noticias_chunk(chunk):
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.executemany("""
                INSERT INTO noticias (id, titulo, resumen, url, fecha, imagen_url, fuente_nombre, categoria_id, pais_id)
                VALUES (%(id)s, %(titulo)s, %(resumen)s, %(url)s, %(fecha)s, %(imagen_url)s, %(fuente_nombre)s, %(categoria_id)s, %(pais_id)s)
                ON CONFLICT (id) DO UPDATE SET
                    titulo = EXCLUDED.titulo,
                    resumen = EXCLUDED.resumen
            """, chunk)
        conn.commit()

def _import_traducciones_chunk(chunk):
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.executemany("""
                INSERT INTO traducciones (id, noticia_id, lang_from, lang_to, titulo_trad, resumen_trad, status, created_at)
                VALUES (%(id)s, %(noticia_id)s, %(lang_from)s, %(lang_to)s, %(titulo_trad)s, %(resumen_trad)s, %(status)s, %(created_at)s)
                ON CONFLICT (id) DO UPDATE SET
                    titulo_trad = EXCLUDED.titulo_trad,
                    resumen_trad = EXCLUDED.resumen_trad
            """, chunk)
        conn.commit()

@config_bp.route("/translator")
def translator_config():
    return "Pagina de configuracion del modelo (pendiente de implementar)"
