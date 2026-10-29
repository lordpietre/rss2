"""Acceso a Postgres del subsistema TTS.

Postgres guarda estado y metadatos. Los archivos de audio viven en el volumen
persistente; aqui solo se guarda su ruta relativa.

Convenciones heredadas de RSS2:

- SQL en crudo con parametros posicionales ($1, $2, ...).
- ``CHAR(5)`` se rellena con espacios: siempre se recorta con TRIM.
"""

from __future__ import annotations

import json
from typing import Any, Dict, List, Optional

from .config import TTSConfig
from .schema import ensure_schema  # re-export

JOB_QUEUED = "queued"
JOB_RUNNING = "running"
JOB_SUCCEEDED = "succeeded"
JOB_FAILED = "failed"
JOB_CANCELLED = "cancelled"

SCRIPT_DRAFT = "draft"
SCRIPT_APPROVED = "approved"
SCRIPT_REJECTED = "rejected"

AUDIO_PENDING_REVIEW = "pending_review"
AUDIO_APPROVED = "approved"
AUDIO_REJECTED = "rejected"

BULLETIN_PENDING = "pending"
BULLETIN_READY = "ready"
BULLETIN_FAILED = "failed"
BULLETIN_APPROVED = "approved"
BULLETIN_REJECTED = "rejected"


def connect(cfg: TTSConfig):
    """Abre una conexion a Postgres."""
    import psycopg2

    return psycopg2.connect(**cfg.db_config)


def _dict_cursor(conn):
    import psycopg2.extras

    return conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)


# ---------------------------------------------------------------------------
# Voces
# ---------------------------------------------------------------------------


def list_voices(conn, only_usable: bool = False) -> List[Dict[str, Any]]:
    query = """
        SELECT id, slug, display_name, engine, TRIM(language) AS language,
               speaker_id, reference_file, duration_seconds, sample_rate,
               quality_status, quality_report, is_builtin, created_at, updated_at
        FROM tts_voices
        ORDER BY is_builtin DESC, slug
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query)
        rows = [dict(row) for row in cur.fetchall()]
    if only_usable:
        rows = [row for row in rows if row.get("quality_status") in ("ok", "unchecked")]
    return rows


def get_voice(conn, voice_id: int) -> Optional[Dict[str, Any]]:
    query = """
        SELECT id, slug, display_name, engine, TRIM(language) AS language,
               speaker_id, reference_file, duration_seconds, sample_rate,
               quality_status, quality_report, is_builtin, created_at, updated_at
        FROM tts_voices WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (voice_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def get_voice_by_slug(conn, slug: str) -> Optional[Dict[str, Any]]:
    query = """
        SELECT id, slug, display_name, engine, TRIM(language) AS language,
               speaker_id, reference_file, duration_seconds, sample_rate,
               quality_status, quality_report, is_builtin, created_at, updated_at
        FROM tts_voices WHERE slug = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (slug,))
        row = cur.fetchone()
    return dict(row) if row else None


def insert_voice(
    conn,
    slug: str,
    display_name: str,
    engine: str,
    language: str = "es",
    speaker_id: Optional[str] = None,
    reference_file: Optional[str] = None,
    duration_seconds: Optional[float] = None,
    sample_rate: Optional[int] = None,
    quality_status: str = "unchecked",
    quality_report: Optional[Dict[str, Any]] = None,
    is_builtin: bool = False,
) -> int:
    query = """
        INSERT INTO tts_voices (slug, display_name, engine, language, speaker_id,
                                reference_file, duration_seconds, sample_rate,
                                quality_status, quality_report, is_builtin)
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        ON CONFLICT (slug) DO UPDATE SET
            display_name = EXCLUDED.display_name,
            engine = EXCLUDED.engine,
            language = EXCLUDED.language,
            speaker_id = EXCLUDED.speaker_id,
            reference_file = EXCLUDED.reference_file,
            duration_seconds = EXCLUDED.duration_seconds,
            sample_rate = EXCLUDED.sample_rate,
            quality_status = EXCLUDED.quality_status,
            quality_report = EXCLUDED.quality_report,
            is_builtin = EXCLUDED.is_builtin,
            updated_at = NOW()
        RETURNING id
    """
    with _dict_cursor(conn) as cur:
        cur.execute(
            query,
            (
                slug,
                display_name,
                engine,
                language,
                speaker_id,
                reference_file,
                duration_seconds,
                sample_rate,
                quality_status,
                json.dumps(quality_report) if quality_report else None,
                is_builtin,
            ),
        )
        row = cur.fetchone()
    conn.commit()
    return int(row["id"])


def delete_voice(conn, voice_id: int) -> bool:
    with _dict_cursor(conn) as cur:
        cur.execute("DELETE FROM tts_voices WHERE id = %s", (voice_id,))
        deleted = cur.rowcount > 0
    conn.commit()
    return deleted


def update_voice_engine(conn, voice_id: int, engine: str) -> bool:
    """Cambia el motor de una voz sin tocar el resto de sus datos.

    Solo lo usa la voz base: la voz por defecto sigue al motor por defecto
    del sistema. Ningun endpoint la expone.
    """
    with _dict_cursor(conn) as cur:
        cur.execute(
            "UPDATE tts_voices SET engine = %s, updated_at = NOW() WHERE id = %s",
            (engine, voice_id),
        )
        updated = cur.rowcount > 0
    conn.commit()
    return updated


# ---------------------------------------------------------------------------
# Guiones
# ---------------------------------------------------------------------------


def insert_script(
    conn,
    noticia_id: Optional[str],
    translation_id: Optional[int],
    language: str,
    title: Optional[str],
    raw_content: str,
    content: str,
    source_title: Optional[str] = None,
    source_url: Optional[str] = None,
    source_name: Optional[str] = None,
    status: str = SCRIPT_DRAFT,
    created_by: Optional[int] = None,
) -> int:
    query = """
        INSERT INTO tts_scripts (noticia_id, translation_id, language, title,
                                 raw_content, content, source_title, source_url,
                                 source_name, status, created_by)
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        RETURNING id
    """
    with _dict_cursor(conn) as cur:
        cur.execute(
            query,
            (
                noticia_id,
                translation_id,
                language,
                title,
                raw_content,
                content,
                source_title,
                source_url,
                source_name,
                status,
                created_by,
            ),
        )
        row = cur.fetchone()
    conn.commit()
    return int(row["id"])


def get_script(conn, script_id: int) -> Optional[Dict[str, Any]]:
    query = """
        SELECT id, noticia_id, translation_id, TRIM(language) AS language, title,
               raw_content, content, source_title, source_url, source_name,
               status, created_by, approved_by, approved_at, created_at, updated_at
        FROM tts_scripts WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (script_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def list_scripts(
    conn,
    status: Optional[str] = None,
    noticia_id: Optional[str] = None,
    limit: int = 50,
    offset: int = 0,
) -> List[Dict[str, Any]]:
    clauses = []
    params: List[Any] = []
    if status:
        clauses.append("status = %s")
        params.append(status)
    if noticia_id:
        clauses.append("noticia_id = %s")
        params.append(noticia_id)
    where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
    params.extend([limit, offset])

    query = """
        SELECT id, noticia_id, translation_id, TRIM(language) AS language, title,
               source_title, source_url, source_name, status, created_by,
               approved_by, approved_at, created_at,
               length(content) AS content_length
        FROM tts_scripts %s
        ORDER BY created_at DESC
        LIMIT %%s OFFSET %%s
    """ % where
    with _dict_cursor(conn) as cur:
        cur.execute(query, params)
        rows = [dict(row) for row in cur.fetchall()]
    return rows


def set_script_status(conn, script_id: int, status: str, user_id: Optional[int] = None) -> bool:
    query = """
        UPDATE tts_scripts
        SET status = %s,
            approved_by = CASE WHEN %s = 'approved' THEN %s ELSE approved_by END,
            approved_at = CASE WHEN %s = 'approved' THEN NOW() ELSE approved_at END,
            updated_at = NOW()
        WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (status, status, user_id, status, script_id))
        updated = cur.rowcount > 0
    conn.commit()
    return updated


# ---------------------------------------------------------------------------
# Tareas
# ---------------------------------------------------------------------------


def insert_job(
    conn,
    script_id: int,
    voice_id: Optional[int],
    engine: str,
    idempotency_key: Optional[str] = None,
    max_attempts: int = 2,
) -> Dict[str, Any]:
    """Crea una tarea. Si la clave de idempotencia ya existe, la devuelve.

    Devuelve ``{"id": ..., "created": bool}``.
    """
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            INSERT INTO tts_jobs (script_id, voice_id, engine, idempotency_key, max_attempts)
            VALUES (%s, %s, %s, %s, %s)
            ON CONFLICT (idempotency_key) DO NOTHING
            RETURNING id
            """,
            (script_id, voice_id, engine, idempotency_key, max_attempts),
        )
        row = cur.fetchone()
        if row:
            conn.commit()
            return {"id": int(row["id"]), "created": True}

        if idempotency_key:
            cur.execute(
                "SELECT id FROM tts_jobs WHERE idempotency_key = %s",
                (idempotency_key,),
            )
            existing = cur.fetchone()
            conn.commit()
            if existing:
                return {"id": int(existing["id"]), "created": False}

    conn.rollback()
    return {"id": None, "created": False}


def get_job(conn, job_id: int) -> Optional[Dict[str, Any]]:
    query = """
        SELECT id, script_id, voice_id, engine, status, idempotency_key, attempts,
               max_attempts, error_code, error_message, queued_at, started_at,
               finished_at, created_at, updated_at
        FROM tts_jobs WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (job_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def list_jobs(
    conn,
    status: Optional[str] = None,
    script_id: Optional[int] = None,
    limit: int = 50,
    offset: int = 0,
) -> List[Dict[str, Any]]:
    clauses = []
    params: List[Any] = []
    if status:
        clauses.append("status = %s")
        params.append(status)
    if script_id is not None:
        clauses.append("script_id = %s")
        params.append(script_id)
    where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
    params.extend([limit, offset])

    query = """
        SELECT id, script_id, voice_id, engine, status, attempts, max_attempts,
               error_code, error_message, queued_at, started_at, finished_at,
               created_at, updated_at
        FROM tts_jobs %s
        ORDER BY created_at DESC
        LIMIT %%s OFFSET %%s
    """ % where
    with _dict_cursor(conn) as cur:
        cur.execute(query, params)
        rows = [dict(row) for row in cur.fetchall()]
    return rows


def claim_job(conn, job_id: int) -> Optional[Dict[str, Any]]:
    """Marca una tarea como 'running' si estaba en 'queued'."""
    query = """
        UPDATE tts_jobs
        SET status = 'running', started_at = NOW(), attempts = attempts + 1, updated_at = NOW()
        WHERE id = %s AND status = 'queued'
        RETURNING id, script_id, voice_id, engine, speed, attempts, max_attempts
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (job_id,))
        row = cur.fetchone()
    conn.commit()
    return dict(row) if row else None


def claim_next_job(conn) -> Optional[Dict[str, Any]]:
    """Toma la siguiente tarea en cola de forma atomica (SKIP LOCKED)."""
    query = """
        UPDATE tts_jobs
        SET status = 'running', started_at = NOW(), attempts = attempts + 1, updated_at = NOW()
        WHERE id = (
            SELECT id FROM tts_jobs
            WHERE status = 'queued'
            ORDER BY queued_at, id
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        RETURNING id, script_id, voice_id, engine, speed, attempts, max_attempts
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query)
        row = cur.fetchone()
    conn.commit()
    return dict(row) if row else None


def mark_job_succeeded(conn, job_id: int) -> None:
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = 'succeeded', finished_at = NOW(), updated_at = NOW(),
                error_code = NULL, error_message = NULL
            WHERE id = %s
            """,
            (job_id,),
        )
    conn.commit()


def mark_job_failed(conn, job_id: int, code: str, message: str, requeue: bool = False) -> None:
    """Marca la tarea como fallida. Con ``requeue`` vuelve a la cola."""
    status = "queued" if requeue else "failed"
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = %s, finished_at = CASE WHEN %s THEN NULL ELSE NOW() END,
                started_at = CASE WHEN %s THEN NULL ELSE started_at END,
                error_code = %s, error_message = %s, updated_at = NOW()
            WHERE id = %s
            """,
            (status, requeue, requeue, code, message[:2000], job_id),
        )
    conn.commit()


def cancel_job(conn, job_id: int) -> bool:
    """Cancela una tarea en cola. Solo es seguro si no ha empezado."""
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = 'cancelled', finished_at = NOW(), updated_at = NOW()
            WHERE id = %s AND status = 'queued'
            """,
            (job_id,),
        )
        cancelled = cur.rowcount > 0
    conn.commit()
    return cancelled


def recover_stale_jobs(
    conn,
    timeout_seconds: int,
    chars_per_second: float = 16.6,
    safety_factor: float = 3.0,
) -> Dict[str, int]:
    """Gestiona las tareas que se quedan atascadas.

    Una tarea puede quedarse en 'running' si el worker muere (una caida, un
    OOM) o si el motor se cuelga. El plazo de cada tarea se escala con el
    tamano del texto: una noticia larga tarda legitimamente mas que el plazo
    fijo, asi que no se reencola trabajo que esta advancing.

    Devuelve cuantas tareas se han reencolado y cuantas se han marcado como
    fallidas (agotaron sus reintentos).
    """
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            SELECT j.id, j.attempts, j.max_attempts,
                   COALESCE(length(sc.content), 0) AS chars,
                   EXTRACT(EPOCH FROM (NOW() - j.started_at)) AS elapsed
            FROM tts_jobs j
            LEFT JOIN tts_scripts sc ON sc.id = j.script_id
            WHERE j.status = 'running' AND j.started_at IS NOT NULL
            """
        )
        rows = [dict(row) for row in cur.fetchall()]

    requeued: List[int] = []
    failed: List[int] = []
    for row in rows:
        estimated = (float(row.get("chars") or 0) / max(chars_per_second, 1.0)) * safety_factor
        deadline = max(float(timeout_seconds), estimated)
        if float(row.get("elapsed") or 0) <= deadline:
            continue
        if int(row.get("attempts") or 0) < int(row.get("max_attempts") or 1):
            requeued.append(int(row["id"]))
        else:
            failed.append(int(row["id"]))

    if requeued:
        with _dict_cursor(conn) as cur:
            cur.execute(
                """
                UPDATE tts_jobs
                SET status = 'queued', started_at = NULL, updated_at = NOW(),
                    error_code = 'stale_recovered',
                    error_message = 'La tarea se quedo atascada y se reencolo'
                WHERE id = ANY(%s)
                """,
                (requeued,),
            )
    if failed:
        with _dict_cursor(conn) as cur:
            cur.execute(
                """
                UPDATE tts_jobs
                SET status = 'failed', finished_at = NOW(), updated_at = NOW(),
                    error_code = 'timeout',
                    error_message = 'La tarea supero el tiempo maximo de ejecucion'
                WHERE id = ANY(%s)
                """,
                (failed,),
            )
    if requeued or failed:
        conn.commit()
    else:
        conn.rollback()

    return {"requeued": len(requeued), "failed": len(failed)}


# ---------------------------------------------------------------------------
# Audios
# ---------------------------------------------------------------------------


def insert_audio(
    conn,
    job_id: int,
    script_id: int,
    voice_id: Optional[int],
    engine: str,
    engine_version: Optional[str],
    storage_path: str,
    duration_seconds: float,
    sample_rate: int,
    size_bytes: int,
    sha256: Optional[str],
    parameters: Optional[Dict[str, Any]] = None,
    status: str = AUDIO_PENDING_REVIEW,
) -> int:
    query = """
        INSERT INTO tts_audios (job_id, script_id, voice_id, engine, engine_version,
                                storage_path, format, duration_seconds, sample_rate,
                                size_bytes, sha256, parameters, status)
        VALUES (%s, %s, %s, %s, %s, %s, 'wav', %s, %s, %s, %s, %s, %s)
        RETURNING id
    """
    with _dict_cursor(conn) as cur:
        cur.execute(
            query,
            (
                job_id,
                script_id,
                voice_id,
                engine,
                engine_version,
                storage_path,
                duration_seconds,
                sample_rate,
                size_bytes,
                sha256,
                json.dumps(parameters or {}),
                status,
            ),
        )
        row = cur.fetchone()
    conn.commit()
    return int(row["id"])


def get_audio(conn, audio_id: int) -> Optional[Dict[str, Any]]:
    query = """
        SELECT a.id, a.job_id, a.script_id, a.voice_id, a.engine, a.engine_version,
               a.storage_path, a.format, a.duration_seconds, a.sample_rate,
               a.size_bytes, a.sha256, a.parameters, a.status, a.approved_by,
               a.approved_at, a.created_at,
               s.noticia_id, s.source_url, s.source_title, s.source_name,
               s.title AS script_title
        FROM tts_audios a
        JOIN tts_scripts s ON s.id = a.script_id
        WHERE a.id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (audio_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def get_audio_by_job(conn, job_id: int) -> Optional[Dict[str, Any]]:
    query = """
        SELECT id, job_id, script_id, voice_id, engine, engine_version, storage_path,
               format, duration_seconds, sample_rate, size_bytes, sha256, parameters,
               status, approved_by, approved_at, created_at
        FROM tts_audios WHERE job_id = %s ORDER BY id DESC LIMIT 1
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (job_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def get_latest_audio_for_script(conn, script_id: int) -> Optional[Dict[str, Any]]:
    """Ultimo audio generado para un guion (el que usaria un noticiario)."""
    query = """
        SELECT id, job_id, script_id, voice_id, engine, engine_version, storage_path,
               format, duration_seconds, sample_rate, size_bytes, sha256, parameters,
               status, created_at
        FROM tts_audios WHERE script_id = %s ORDER BY id DESC LIMIT 1
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (script_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def list_audios(
    conn,
    status: Optional[str] = None,
    script_id: Optional[int] = None,
    limit: int = 50,
    offset: int = 0,
) -> List[Dict[str, Any]]:
    clauses = []
    params: List[Any] = []
    if status:
        clauses.append("a.status = %s")
        params.append(status)
    if script_id is not None:
        clauses.append("a.script_id = %s")
        params.append(script_id)
    where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
    params.extend([limit, offset])

    query = """
        SELECT a.id, a.job_id, a.script_id, a.voice_id, a.engine, a.engine_version,
               a.storage_path, a.format, a.duration_seconds, a.sample_rate,
               a.size_bytes, a.status, a.approved_by, a.approved_at, a.created_at,
               s.noticia_id, s.source_url, s.source_name
        FROM tts_audios a
        JOIN tts_scripts s ON s.id = a.script_id
        %s
        ORDER BY a.created_at DESC
        LIMIT %%s OFFSET %%s
    """ % where
    with _dict_cursor(conn) as cur:
        cur.execute(query, params)
        rows = [dict(row) for row in cur.fetchall()]
    return rows


def set_audio_status(conn, audio_id: int, status: str, user_id: Optional[int] = None) -> bool:
    query = """
        UPDATE tts_audios
        SET status = %s,
            approved_by = CASE WHEN %s IN ('approved', 'rejected') THEN %s ELSE approved_by END,
            approved_at = CASE WHEN %s IN ('approved', 'rejected') THEN NOW() ELSE approved_at END
        WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (status, status, user_id, status, audio_id))
        updated = cur.rowcount > 0
    conn.commit()
    return updated


# ---------------------------------------------------------------------------
# Noticiario (edicion de varias narraciones)
# ---------------------------------------------------------------------------


def insert_bulletin(
    conn,
    title: str,
    language: str = "es",
    silence_ms: int = 800,
    created_by: Optional[int] = None,
) -> int:
    query = """
        INSERT INTO tts_bulletins (title, language, silence_ms, created_by)
        VALUES (%s, %s, %s, %s)
        RETURNING id
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (title, language, silence_ms, created_by))
        row = cur.fetchone()
    conn.commit()
    return int(row["id"])


def insert_bulletin_items(conn, bulletin_id: int, items: List[Dict[str, Any]]) -> None:
    """Inserta los items del noticiario en orden.

    Cada item apunta a un guion ya aprobado; la procedencia se copia aqui
    para que el manifiesto del noticiario sea autonomo.
    """
    if not items:
        return
    query = """
        INSERT INTO tts_bulletin_items (bulletin_id, script_id, position, source_url,
                                        source_name, source_title)
        VALUES (%s, %s, %s, %s, %s, %s)
    """
    with _dict_cursor(conn) as cur:
        for position, item in enumerate(items, start=1):
            cur.execute(
                query,
                (
                    bulletin_id,
                    item["script_id"],
                    position,
                    item.get("source_url"),
                    item.get("source_name"),
                    item.get("source_title"),
                ),
            )
        cur.execute(
            "UPDATE tts_bulletins SET item_count = %s, updated_at = NOW() WHERE id = %s",
            (len(items), bulletin_id),
        )
    conn.commit()


def get_bulletin(conn, bulletin_id: int) -> Optional[Dict[str, Any]]:
    query = """
        SELECT id, title, TRIM(language) AS language, status, audio_path,
               duration_seconds, size_bytes, sha256, item_count, silence_ms,
               created_by, approved_by, approved_at, created_at, updated_at
        FROM tts_bulletins WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (bulletin_id,))
        row = cur.fetchone()
    return dict(row) if row else None


def list_bulletins(conn, status: Optional[str] = None, limit: int = 50, offset: int = 0) -> List[Dict[str, Any]]:
    clauses = []
    params: List[Any] = []
    if status:
        clauses.append("status = %s")
        params.append(status)
    where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
    params.extend([limit, offset])

    query = """
        SELECT id, title, TRIM(language) AS language, status, audio_path,
               duration_seconds, size_bytes, sha256, item_count, silence_ms,
               created_by, approved_by, approved_at, created_at, updated_at
        FROM tts_bulletins %s
        ORDER BY created_at DESC
        LIMIT %%s OFFSET %%s
    """ % where
    with _dict_cursor(conn) as cur:
        cur.execute(query, params)
        return [dict(row) for row in cur.fetchall()]


def get_bulletin_items(conn, bulletin_id: int) -> List[Dict[str, Any]]:
    query = """
        SELECT i.id, i.bulletin_id, i.script_id, i.position, i.audio_id,
               i.audio_path, i.duration_seconds, i.start_offset,
               i.source_url, i.source_name, i.source_title,
               s.noticia_id, s.title AS script_title, s.status AS script_status
        FROM tts_bulletin_items i
        JOIN tts_scripts s ON s.id = i.script_id
        WHERE i.bulletin_id = %s
        ORDER BY i.position
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (bulletin_id,))
        return [dict(row) for row in cur.fetchall()]


def get_pending_bulletins_for_script(conn, script_id: int) -> List[Dict[str, Any]]:
    """Noticiarios a la espera de la narracion de un guion concreto."""
    query = """
        SELECT b.id, b.title, TRIM(b.language) AS language, b.silence_ms
        FROM tts_bulletins b
        JOIN tts_bulletin_items i ON i.bulletin_id = b.id
        WHERE i.script_id = %s AND b.status = 'pending'
        ORDER BY b.id
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (script_id,))
        return [dict(row) for row in cur.fetchall()]


def set_bulletin_item_audio(
    conn,
    bulletin_id: int,
    script_id: int,
    audio_id: int,
    audio_path: str,
    duration_seconds: float,
    start_offset: Optional[float] = None,
) -> None:
    query = """
        UPDATE tts_bulletin_items
        SET audio_id = COALESCE(NULLIF(%s, 0), audio_id),
            audio_path = COALESCE(NULLIF(%s, ''), audio_path),
            duration_seconds = %s,
            start_offset = COALESCE(%s, start_offset)
        WHERE bulletin_id = %s AND script_id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (audio_id, audio_path, duration_seconds, start_offset, bulletin_id, script_id))
    conn.commit()


def update_bulletin_audio(
    conn,
    bulletin_id: int,
    status: str,
    audio_path: Optional[str] = None,
    duration_seconds: Optional[float] = None,
    size_bytes: Optional[int] = None,
    sha256: Optional[str] = None,
) -> None:
    query = """
        UPDATE tts_bulletins
        SET status = %s,
            audio_path = COALESCE(%s, audio_path),
            duration_seconds = COALESCE(%s, duration_seconds),
            size_bytes = COALESCE(%s, size_bytes),
            sha256 = COALESCE(%s, sha256),
            updated_at = NOW()
        WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (status, audio_path, duration_seconds, size_bytes, sha256, bulletin_id))
    conn.commit()


def set_bulletin_status(conn, bulletin_id: int, status: str, user_id: Optional[int] = None) -> bool:
    query = """
        UPDATE tts_bulletins
        SET status = %s,
            approved_by = CASE WHEN %s IN ('approved', 'rejected') THEN %s ELSE approved_by END,
            approved_at = CASE WHEN %s IN ('approved', 'rejected') THEN NOW() ELSE approved_at END,
            updated_at = NOW()
        WHERE id = %s
    """
    with _dict_cursor(conn) as cur:
        cur.execute(query, (status, status, user_id, status, bulletin_id))
        updated = cur.rowcount > 0
    conn.commit()
    return updated
