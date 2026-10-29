"""Pruebas de la API interna del servicio de TTS.

Incluyen la proteccion de los endpoints (9): autenticacion por clave y
ausencia de rutas internas en las respuestas.
"""

from __future__ import annotations

import io
import os

import numpy as np
import pytest
import soundfile

from workers.tts import db as ttsdb
from workers.tests.conftest import requires_db

pytestmark = requires_db


@pytest.fixture
def client(service):
    app = service.create_app()
    app.config["TESTING"] = True
    return app.test_client()


def wav_bytes(seconds: float = 6.0, sample_rate: int = 22050) -> bytes:
    buffer = io.BytesIO()
    t = np.linspace(0, seconds, int(seconds * sample_rate), endpoint=False, dtype=np.float32)
    signal = (0.5 * np.sin(2 * np.pi * 220 * t)).astype(np.float32)
    soundfile.write(buffer, signal, sample_rate, format="WAV")
    return buffer.getvalue()


# ---------------------------------------------------------------------------
# Salud
# ---------------------------------------------------------------------------


def test_health_reports_the_engines(client):
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.get_json()
    assert payload["status"] == "ok"
    assert "fake" in payload["engines"]
    assert "queue" in payload


# ---------------------------------------------------------------------------
# Normalizacion de guiones
# ---------------------------------------------------------------------------


def test_normalize_endpoint_expands_numbers(client):
    response = client.post(
        "/api/scripts/normalize",
        json={"text": "El paro sube al 12,4% en 2024.", "language": "es"},
    )
    assert response.status_code == 200
    payload = response.get_json()
    assert "por ciento" in payload["text"]
    assert "%" not in payload["text"]
    assert payload["length"] == len(payload["text"])


def test_normalize_endpoint_rejects_a_non_string(client):
    response = client.post("/api/scripts/normalize", json={"text": 12345})
    assert response.status_code == 400
    assert response.get_json()["error"] == "invalid_request"


def test_normalize_endpoint_accepts_empty_text(client):
    response = client.post("/api/scripts/normalize", json={"text": ""})
    assert response.status_code == 200
    assert response.get_json()["text"] == ""


# ---------------------------------------------------------------------------
# Tareas
# ---------------------------------------------------------------------------


def test_unknown_job_returns_404(client):
    response = client.get("/api/jobs/999999")
    assert response.status_code == 404
    assert response.get_json()["error"] == "not_found"


def test_cancel_a_running_job_is_refused(client, service, db_conn):
    script_id = ttsdb.insert_script(
        db_conn,
        noticia_id=None,
        translation_id=None,
        language="es",
        title=None,
        raw_content="texto",
        content="texto",
    )
    job = ttsdb.insert_job(db_conn, script_id=script_id, voice_id=None, engine="fake")
    ttsdb.claim_job(db_conn, job["id"])

    response = client.post("/api/jobs/%d/cancel" % job["id"])
    assert response.status_code == 409
    assert response.get_json()["error"] == "job_running"


def test_cancel_a_queued_job(client, service, db_conn):
    script_id = ttsdb.insert_script(
        db_conn,
        noticia_id=None,
        translation_id=None,
        language="es",
        title=None,
        raw_content="texto",
        content="texto",
    )
    job = ttsdb.insert_job(db_conn, script_id=script_id, voice_id=None, engine="fake")

    response = client.post("/api/jobs/%d/cancel" % job["id"])
    assert response.status_code == 200
    assert response.get_json()["job"]["status"] == "cancelled"


# ---------------------------------------------------------------------------
# Voces
# ---------------------------------------------------------------------------


def test_voice_upload_is_disabled_by_default(client):
    response = client.post(
        "/api/voices",
        data={"sample": (wav_bytes(), "voz.wav"), "slug": "mi-voz"},
    )
    assert response.status_code == 403
    assert response.get_json()["error"] == "voice_upload_disabled"


def test_voice_upload_stores_the_sample_in_the_voices_volume(client, service):
    service.cfg.allow_voice_upload = True
    response = client.post(
        "/api/voices",
        data={
            "sample": (io.BytesIO(wav_bytes()), "mi-voz.wav"),
            "slug": "mi-voz",
            "display_name": "Mi voz",
        },
    )
    assert response.status_code == 201
    payload = response.get_json()
    assert payload["voice"]["slug"] == "mi-voz"
    assert payload["quality"]["ok"] is True

    # La muestra queda en el volumen de voces, no en otro sitio.
    stored = os.path.join(service.cfg.voices_dir, "mi-voz.wav")
    assert os.path.isfile(stored)

    # La respuesta no expone la ruta interna del audio de referencia.
    assert "reference_file" not in payload["voice"]


def test_voice_upload_without_file_is_rejected(client, service):
    service.cfg.allow_voice_upload = True
    response = client.post("/api/voices", data={"slug": "sin-fichero"})
    assert response.status_code == 400
    assert response.get_json()["error"] == "missing_sample"


def test_voice_list_does_not_expose_internal_paths(client, service, db_conn):
    service.cfg.allow_voice_upload = True
    client.post(
        "/api/voices",
        data={"sample": (io.BytesIO(wav_bytes()), "otra-voz.wav"), "slug": "otra-voz"},
    )

    response = client.get("/api/voices")
    assert response.status_code == 200
    for voice in response.get_json()["voices"]:
        assert "reference_file" not in voice


def test_voice_quality_endpoint(client, service):
    service.cfg.allow_voice_upload = True
    created = client.post(
        "/api/voices",
        data={"sample": (io.BytesIO(wav_bytes()), "calidad.wav"), "slug": "calidad"},
    ).get_json()

    response = client.get("/api/voices/%d/quality" % created["voice"]["id"])
    assert response.status_code == 200
    assert response.get_json()["report"]["ok"] is True


def test_delete_voice_removes_the_sample(client, service):
    service.cfg.allow_voice_upload = True
    created = client.post(
        "/api/voices",
        data={"sample": (io.BytesIO(wav_bytes()), "borrable.wav"), "slug": "borrable"},
    ).get_json()
    stored = os.path.join(service.cfg.voices_dir, "borrable.wav")
    assert os.path.isfile(stored)

    response = client.delete("/api/voices/%d" % created["voice"]["id"])
    assert response.status_code == 200
    assert not os.path.exists(stored)


# ---------------------------------------------------------------------------
# 9. Proteccion de los endpoints
# ---------------------------------------------------------------------------


def test_api_key_is_required_when_configured(client, service):
    service.cfg.api_key = "clave-secreta"

    denied = client.get("/api/voices")
    assert denied.status_code == 401
    assert denied.get_json()["error"] == "unauthorized"

    allowed = client.get("/api/voices", headers={"X-API-Key": "clave-secreta"})
    assert allowed.status_code == 200


def test_health_stays_open_for_the_container_healthcheck(client, service):
    """El healthcheck no lleva credenciales: debe seguir accesible."""
    service.cfg.api_key = "clave-secreta"
    response = client.get("/health")
    assert response.status_code == 200


def test_synthesize_endpoint_rejects_empty_text(client):
    response = client.post("/api/synthesize", json={"text": "   "})
    assert response.status_code == 400
    assert response.get_json()["error"] == "empty_text"


def test_synthesize_endpoint_rejects_an_unknown_voice(client):
    response = client.post("/api/synthesize", json={"text": "hola", "voice": "no-existe"})
    assert response.status_code == 404


def test_voice_upload_uses_the_default_engine(client, service):
    """Sin motor pedido, la muestra se registra con el del sistema.

    Antes estaba hardcodeado a "openvoice": la subida debia clonarse con
    Chatterbox, que es quien aprende timbre y acento del audio.
    """
    service.cfg.allow_voice_upload = True
    response = client.post(
        "/api/voices",
        data={"sample": (io.BytesIO(wav_bytes()), "clon.wav"), "slug": "clon"},
    )
    assert response.status_code == 201
    assert response.get_json()["voice"]["engine"] == service.cfg.default_engine


def test_voice_upload_accepts_an_explicit_engine(client, service):
    service.cfg.allow_voice_upload = True
    service.cfg.engines = ["fake", "openvoice"]

    response = client.post(
        "/api/voices",
        data={
            "sample": (io.BytesIO(wav_bytes()), "clasica.wav"),
            "slug": "clasica",
            "engine": "openvoice",
        },
    )
    assert response.status_code == 201
    assert response.get_json()["voice"]["engine"] == "openvoice"


def test_voice_upload_rejects_an_unknown_engine(client, service):
    service.cfg.allow_voice_upload = True

    response = client.post(
        "/api/voices",
        data={
            "sample": (io.BytesIO(wav_bytes()), "rara.wav"),
            "slug": "rara",
            "engine": "speak-jet",
        },
    )
    assert response.status_code == 400
    assert response.get_json()["error"] == "unknown_engine"
    # La muestra no se queda huérfana en el volumen.
    assert not os.path.isfile(os.path.join(service.cfg.voices_dir, "rara.wav"))


# ---------------------------------------------------------------------------
# Formatos de la muestra: WAV, MP3 y OGG
# ---------------------------------------------------------------------------


def _audio_bytes(format: str, subtype=None) -> bytes:
    """Muestra de 6 s codificada en el formato pedido."""
    buffer = io.BytesIO()
    t = np.linspace(0, 6.0, 6 * 22050, endpoint=False, dtype=np.float32)
    signal = (0.5 * np.sin(2 * np.pi * 220 * t)).astype(np.float32)
    kwargs = {"format": format}
    if subtype:
        kwargs["subtype"] = subtype
    soundfile.write(buffer, signal, 22050, **kwargs)
    return buffer.getvalue()


@pytest.mark.parametrize(
    "filename, fmt, subtype",
    [
        ("voz.mp3", "MP3", "MPEG_LAYER_III"),
        ("voz.ogg", "OGG", "VORBIS"),
    ],
)
def test_voice_upload_accepts_mp3_and_ogg(client, service, filename, fmt, subtype):
    """La muestra se guarda con SU extension: soundfile decodifica por ella."""
    service.cfg.allow_voice_upload = True
    slug = filename.split(".")[0] + "-y-" + fmt.lower()

    response = client.post(
        "/api/voices",
        data={"sample": (io.BytesIO(_audio_bytes(fmt, subtype)), filename), "slug": slug},
    )

    assert response.status_code == 201, response.get_json()
    assert response.get_json()["voice"]["engine"] == service.cfg.default_engine
    # El archivo queda con la extension real, no como .wav.
    stored = os.path.join(service.cfg.voices_dir, slug + os.path.splitext(filename)[1])
    assert os.path.isfile(stored)
    # Y se puede decodificar (es lo que hara Chatterbox con la referencia).
    data, sample_rate = soundfile.read(stored)
    assert len(data) > 0 and sample_rate == 22050


def test_voice_upload_rejects_unsupported_formats(client, service):
    service.cfg.allow_voice_upload = True
    response = client.post(
        "/api/voices",
        data={"sample": (io.BytesIO(b"ID3esto no es audio"), "cancion.m4a"), "slug": "cancion"},
    )
    assert response.status_code == 400
    assert response.get_json()["error"] == "unsupported_format"
    # No queda nada de esta muestra en el volumen (el resto de ficheros que
    # haya son las voces integradas).
    assert not [f for f in os.listdir(service.cfg.voices_dir) if f.startswith("cancion")]
