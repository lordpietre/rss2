"""Pruebas de integracion del servicio de TTS.

Cubren los puntos 4, 5, 6 y 8 del plan de aceptacion:
fallo del motor, reinicio con tareas pendientes, reintentos y duplicados, y
la relacion entre audio, guion y noticia original.

Requieren un Postgres de pruebas (TEST_DATABASE_URL). Si no esta disponible,
se saltan.
"""

from __future__ import annotations

import os

import pytest

from workers.tts import db as ttsdb
from workers.tts.engine import EngineError
from workers.tests.conftest import requires_db

pytestmark = requires_db


# ---------------------------------------------------------------------------
# Utilidades
# ---------------------------------------------------------------------------


def create_noticia(conn, noticia_id="tts-noticia-1", url="https://ejemplo.local/tts-1"):
    with conn.cursor() as cur:
        cur.execute(
            "INSERT INTO noticias (id, url) VALUES (%s, %s) ON CONFLICT (id) DO NOTHING",
            (noticia_id, url),
        )
    conn.commit()


def create_script(conn, status=ttsdb.SCRIPT_APPROVED, noticia_id="tts-noticia-1", content="Guion de prueba."):
    script_id = ttsdb.insert_script(
        conn,
        noticia_id=noticia_id,
        translation_id=None,
        language="es",
        title="Titular",
        raw_content=content,
        content=content,
        source_title="Titular original",
        source_url="https://ejemplo.local/tts-1",
        source_name="Medio de prueba",
        status=status,
    )
    return script_id


def create_job(conn, script_id, engine="fake", idempotency_key=None, max_attempts=2):
    return ttsdb.insert_job(
        conn,
        script_id=script_id,
        voice_id=None,
        engine=engine,
        idempotency_key=idempotency_key,
        max_attempts=max_attempts,
    )


def claim(conn, job_id):
    job = ttsdb.claim_job(conn, job_id)
    assert job is not None
    return job


# ---------------------------------------------------------------------------
# 8. Relacion entre audio, guion y noticia original
# ---------------------------------------------------------------------------


def test_audio_keeps_the_link_to_script_and_news(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)
    service._process_job(claim(db_conn, job["id"]))

    audio = ttsdb.get_audio_by_job(db_conn, job["id"])
    assert audio is not None
    assert audio["script_id"] == script_id

    script = ttsdb.get_script(db_conn, script_id)
    assert script["noticia_id"] == "tts-noticia-1"
    # La fuente original se conserva en el guion y en el audio.
    assert script["source_url"] == "https://ejemplo.local/tts-1"
    assert script["source_name"] == "Medio de prueba"

    detail = ttsdb.get_audio(db_conn, audio["id"])
    assert detail["source_url"] == "https://ejemplo.local/tts-1"
    assert detail["noticia_id"] == "tts-noticia-1"


def test_deleting_the_news_removes_its_scripts(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    assert ttsdb.get_script(db_conn, script_id) is not None

    with db_conn.cursor() as cur:
        cur.execute("DELETE FROM noticias WHERE id = %s", ("tts-noticia-1",))
    db_conn.commit()

    assert ttsdb.get_script(db_conn, script_id) is None


def test_audio_file_is_written_in_the_persistent_volume(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)
    service._process_job(claim(db_conn, job["id"]))

    audio = ttsdb.get_audio_by_job(db_conn, job["id"])
    full_path = os.path.join(service.cfg.audio_dir, audio["storage_path"])
    assert os.path.isfile(full_path)
    assert audio["duration_seconds"] == pytest.approx(0.5, abs=0.05)
    assert audio["size_bytes"] > 0
    assert audio["sha256"]


def test_audio_starts_as_pending_review(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)
    service._process_job(claim(db_conn, job["id"]))

    audio = ttsdb.get_audio_by_job(db_conn, job["id"])
    assert audio["status"] == ttsdb.AUDIO_PENDING_REVIEW
    assert audio["approved_by"] is None


# ---------------------------------------------------------------------------
# 4. Fallo del motor TTS
# ---------------------------------------------------------------------------


def test_engine_failure_marks_the_job_as_failed(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id, max_attempts=1)

    engine = service.engines["fake"]
    engine.failure = EngineError("synthesis_failed", "el motor fallo", retryable=False)

    service._process_job(claim(db_conn, job["id"]))

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_FAILED
    assert state["error_code"] == "synthesis_failed"
    assert ttsdb.get_audio_by_job(db_conn, job["id"]) is None


def test_unapproved_script_is_not_synthesized(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn, status=ttsdb.SCRIPT_DRAFT)
    job = create_job(db_conn, script_id)

    service._process_job(claim(db_conn, job["id"]))

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_FAILED
    assert state["error_code"] == "script_not_approved"


def test_unknown_engine_fails_with_a_clear_code(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id, engine="motor-inexistente")

    service._process_job(claim(db_conn, job["id"]))

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_FAILED
    assert state["error_code"] == "unknown_engine"


# ---------------------------------------------------------------------------
# 6. Reintentos y prevencion de duplicados
# ---------------------------------------------------------------------------


def test_retryable_failure_requeues_the_job(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id, max_attempts=3)

    engine = service.engines["fake"]
    engine.failure = EngineError("synthesis_failed", "fallo temporal", retryable=True)

    service._process_job(claim(db_conn, job["id"]))

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_QUEUED
    assert state["error_code"] == "synthesis_failed"


def test_idempotency_key_prevents_duplicate_jobs(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)

    first = create_job(db_conn, script_id, idempotency_key="clave-fija")
    second = create_job(db_conn, script_id, idempotency_key="clave-fija")

    assert first["created"] is True
    assert second["created"] is False
    assert first["id"] == second["id"]

    jobs = ttsdb.list_jobs(db_conn, script_id=script_id)
    assert len(jobs) == 1


def test_a_second_job_does_not_repeat_the_synthesis(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    key = "misma-narracion"

    first = create_job(db_conn, script_id, idempotency_key=key)
    second = create_job(db_conn, script_id, idempotency_key=key)

    service._process_job(claim(db_conn, first["id"]))
    engine = service.engines["fake"]
    assert len(engine.calls) == 1

    # La segunda peticion devuelve la misma tarea: no hay trabajo duplicado.
    assert second["id"] == first["id"]


# ---------------------------------------------------------------------------
# 5. Reinicio del servicio con tareas pendientes
# ---------------------------------------------------------------------------


def test_stale_running_jobs_are_requeued_after_a_restart(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)

    # El worker murio con la tarea en 'running'.
    with db_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = 'running', started_at = NOW() - INTERVAL '2 hours', attempts = 1
            WHERE id = %s
            """,
            (job["id"],),
        )
    db_conn.commit()

    recovered = ttsdb.recover_stale_jobs(db_conn, timeout_seconds=3600)
    assert recovered["requeued"] == 1
    assert recovered["failed"] == 0

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_QUEUED
    assert state["started_at"] is None

    # Y la tarea se puede procesar despues de la recuperacion.
    service._process_job(claim(db_conn, job["id"]))
    assert ttsdb.get_job(db_conn, job["id"])["status"] == ttsdb.JOB_SUCCEEDED


def test_fresh_running_jobs_are_not_requeued(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)

    with db_conn.cursor() as cur:
        cur.execute(
            "UPDATE tts_jobs SET status = 'running', started_at = NOW() WHERE id = %s",
            (job["id"],),
        )
    db_conn.commit()

    recovered = ttsdb.recover_stale_jobs(db_conn, timeout_seconds=3600)
    assert recovered["requeued"] == 0 and recovered["failed"] == 0
    assert ttsdb.get_job(db_conn, job["id"])["status"] == ttsdb.JOB_RUNNING


def test_queued_jobs_survive_a_restart(service, db_conn):
    """Una tarea en cola sigue en la cola: la cola es persistente."""
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)

    # "Reinicio": el servicio vuelve a arrancar y reclama la tarea.
    claimed = ttsdb.claim_next_job(db_conn)
    assert claimed is not None
    assert claimed["id"] == job["id"]
    assert ttsdb.get_job(db_conn, job["id"])["status"] == ttsdb.JOB_RUNNING


def test_claim_next_job_is_exclusive(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    create_job(db_conn, script_id)

    first = ttsdb.claim_next_job(db_conn)
    second = ttsdb.claim_next_job(db_conn)
    assert first is not None
    assert second is None


# ---------------------------------------------------------------------------
# Cancelacion
# ---------------------------------------------------------------------------


def test_cancel_only_works_on_queued_jobs(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id)

    assert ttsdb.cancel_job(db_conn, job["id"]) is True
    assert ttsdb.get_job(db_conn, job["id"])["status"] == ttsdb.JOB_CANCELLED

    # Una tarea ya en marcha no se puede cancelar de forma segura.
    other = create_job(db_conn, script_id)
    claim(db_conn, other["id"])
    assert ttsdb.cancel_job(db_conn, other["id"]) is False


# ---------------------------------------------------------------------------
# Noticiario
# ---------------------------------------------------------------------------


def test_bulletin_waits_until_all_its_pieces_are_ready(service, db_conn):
    """Un noticiario no se monta hasta que estan todas sus narraciones."""
    create_noticia(db_conn)
    script_a = create_script(db_conn, content="Primera noticia del noticiario.")
    script_b = create_script(db_conn, content="Segunda noticia del noticiario.")

    bulletin_id = ttsdb.insert_bulletin(db_conn, title="Edicion de prueba", language="es", silence_ms=200)
    ttsdb.insert_bulletin_items(
        db_conn,
        bulletin_id,
        [
            {"script_id": script_a, "source_url": "https://ejemplo.local/a", "source_name": "Medio A"},
            {"script_id": script_b, "source_url": "https://ejemplo.local/b", "source_name": "Medio B"},
        ],
    )

    # Todavia no hay ninguna narracion: el noticiario sigue pendiente.
    assert service.build_bulletin(bulletin_id)["status"] == ttsdb.BULLETIN_PENDING

    job_a = create_job(db_conn, script_a)
    service._process_job(claim(db_conn, job_a["id"]))
    assert ttsdb.get_bulletin(db_conn, bulletin_id)["status"] == ttsdb.BULLETIN_PENDING

    job_b = create_job(db_conn, script_b)
    service._process_job(claim(db_conn, job_b["id"]))

    bulletin = service.build_bulletin(bulletin_id)
    assert bulletin["status"] == ttsdb.BULLETIN_READY
    assert bulletin["duration_seconds"] > 0
    assert bulletin["audio_path"].startswith("bulletins/")

    # El archivo existe y es un WAV valido.
    full_path = os.path.join(service.cfg.audio_dir, bulletin["audio_path"])
    assert os.path.isfile(full_path)
    from workers.tts.engine import probe_audio

    meta = probe_audio(full_path)
    assert meta["duration_seconds"] > 0

    # El manifiesto conserva el origen de cada noticia.
    items = ttsdb.get_bulletin_items(db_conn, bulletin_id)
    assert [item["position"] for item in items] == [1, 2]
    assert items[0]["source_url"] == "https://ejemplo.local/a"
    assert items[1]["source_url"] == "https://ejemplo.local/b"
    assert items[0]["noticia_id"] == "tts-noticia-1"


def test_bulletin_items_keep_the_order_of_the_scripts(service, db_conn):
    create_noticia(db_conn)
    script_a = create_script(db_conn)
    script_b = create_script(db_conn)

    bulletin_id = ttsdb.insert_bulletin(db_conn, title="Orden", language="es")
    ttsdb.insert_bulletin_items(
        db_conn,
        bulletin_id,
        [{"script_id": script_b}, {"script_id": script_a}],
    )

    items = ttsdb.get_bulletin_items(db_conn, bulletin_id)
    assert [item["script_id"] for item in items] == [script_b, script_a]
    assert [item["position"] for item in items] == [1, 2]


def test_pending_bulletins_are_found_by_script(service, db_conn):
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    bulletin_id = ttsdb.insert_bulletin(db_conn, title="Busqueda", language="es")
    ttsdb.insert_bulletin_items(db_conn, bulletin_id, [{"script_id": script_id}])

    pending = ttsdb.get_pending_bulletins_for_script(db_conn, script_id)
    assert [b["id"] for b in pending] == [bulletin_id]

    # Un noticiario ya montado deja de esperar.
    ttsdb.update_bulletin_audio(db_conn, bulletin_id, ttsdb.BULLETIN_READY)
    assert ttsdb.get_pending_bulletins_for_script(db_conn, script_id) == []


# ---------------------------------------------------------------------------
# 5. Reinicio del servicio con tareas pendientes (vigilante)
# ---------------------------------------------------------------------------


def test_watchdog_requeues_a_stuck_job(service, db_conn):
    """Una tarea atascada se reencola si le quedan reintentos."""
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id, max_attempts=2)

    with db_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = 'running', started_at = NOW() - INTERVAL '3 hours', attempts = 1
            WHERE id = %s
            """,
            (job["id"],),
        )
    db_conn.commit()

    result = ttsdb.recover_stale_jobs(db_conn, timeout_seconds=3600)
    assert result["requeued"] == 1
    assert result["failed"] == 0

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_QUEUED
    assert state["started_at"] is None
    assert state["error_code"] == "stale_recovered"


def test_watchdog_fails_a_job_without_retries_left(service, db_conn):
    """Si agotó los reintentos, la tarea se falla (no se reencola para siempre)."""
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id, max_attempts=1)

    with db_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = 'running', started_at = NOW() - INTERVAL '3 hours', attempts = 1
            WHERE id = %s
            """,
            (job["id"],),
        )
    db_conn.commit()

    result = ttsdb.recover_stale_jobs(db_conn, timeout_seconds=3600)
    assert result["failed"] == 1
    assert result["requeued"] == 0

    state = ttsdb.get_job(db_conn, job["id"])
    assert state["status"] == ttsdb.JOB_FAILED
    assert state["error_code"] == "timeout"


def test_watchdog_leaves_a_healthy_job_alone(service, db_conn):
    """Una tarea que acaba de empezar no se toca."""
    create_noticia(db_conn)
    script_id = create_script(db_conn)
    job = create_job(db_conn, script_id, max_attempts=2)

    with db_conn.cursor() as cur:
        cur.execute(
            "UPDATE tts_jobs SET status = 'running', started_at = NOW() WHERE id = %s",
            (job["id"],),
        )
    db_conn.commit()

    result = ttsdb.recover_stale_jobs(db_conn, timeout_seconds=3600)
    assert result["requeued"] == 0
    assert result["failed"] == 0
    assert ttsdb.get_job(db_conn, job["id"])["status"] == ttsdb.JOB_RUNNING


def test_watchdog_scales_the_deadline_with_the_text_length(service, db_conn):
    """Una noticia larga tarda mas: no se reencola mientras avanza."""
    create_noticia(db_conn)
    long_text = "palabra " * 2000  # ~16.000 caracteres
    script_id = create_script(db_conn, content=long_text)
    job = create_job(db_conn, script_id, max_attempts=2)

    with db_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE tts_jobs
            SET status = 'running', started_at = NOW() - INTERVAL '20 minutes', attempts = 1
            WHERE id = %s
            """,
            (job["id"],),
        )
    db_conn.commit()

    # 16.000 chars / 16.6 chars/s * 3 = ~2.900 s de plazo: 20 min (1.200 s)
    # no lo supera todavia.
    result = ttsdb.recover_stale_jobs(db_conn, timeout_seconds=3600)
    assert result["requeued"] == 0
    assert ttsdb.get_job(db_conn, job["id"])["status"] == ttsdb.JOB_RUNNING


# ---------------------------------------------------------------------------
# Voces integradas: la voz base sigue al motor por defecto
# ---------------------------------------------------------------------------


def test_builtin_voice_follows_the_default_engine(service, db_conn):
    """``es-base`` es la voz del motor por defecto, no de un motor fijo.

    Cambiar ``TTS_ENGINE`` cambia la voz base. Las voces que declaran otro
    motor (hablantes de MeloTTS, ``ov-*`` de OpenVoice) no se tocan.
    """
    base = ttsdb.get_voice_by_slug(db_conn, "es-base")
    assert base is not None
    assert base["engine"] == service.cfg.default_engine

    # Una voz con motor propio queda como esta.
    ttsdb.insert_voice(
        db_conn,
        slug="ov-prueba",
        display_name="Voz de prueba",
        engine="openvoice",
        language="es",
        is_builtin=True,
    )

    service._seed_builtin_voices()

    assert ttsdb.get_voice_by_slug(db_conn, "es-base")["engine"] == service.cfg.default_engine
    assert ttsdb.get_voice_by_slug(db_conn, "ov-prueba")["engine"] == "openvoice"
