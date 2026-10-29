"""Configuracion del servicio de TTS.

Todo se lee del entorno (docker compose / .env). No hay rutas absolutas de
desarrollo ni credenciales en el repositorio: los pesos del modelo y las
muestras de voz propias viven en volumenes montados.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import List


def _env_str(key: str, default: str = "") -> str:
    value = os.environ.get(key)
    if value is None:
        return default
    value = value.strip()
    return value if value else default


def _env_int(key: str, default: int) -> int:
    try:
        raw = os.environ.get(key, "").strip()
        return int(raw) if raw else default
    except (TypeError, ValueError):
        return default


def _env_float(key: str, default: float) -> float:
    try:
        raw = os.environ.get(key, "").strip()
        return float(raw) if raw else default
    except (TypeError, ValueError):
        return default


def _env_bool(key: str, default: bool) -> bool:
    raw = os.environ.get(key)
    if raw is None or raw.strip() == "":
        return default
    return raw.strip().lower() in ("1", "true", "yes", "on", "si")


def _env_list(key: str, default: str = "") -> List[str]:
    raw = _env_str(key, default)
    return [item.strip() for item in raw.split(",") if item.strip()]


@dataclass
class TTSConfig:
    """Configuracion efectiva del servicio.

    Los valores por defecto apuntan a rutas *dentro del contenedor*; el host las
    sobreescribe con el compose. Ninguna ruta de desarrollo queda incrustada.
    """

    # --- Base de datos -----------------------------------------------------
    db_host: str = "db"
    db_port: int = 5432
    db_name: str = "rss"
    db_user: str = "rss"
    db_pass: str = ""
    # Si se define, tiene prioridad sobre db_host/port/... (formato URL).
    db_url: str = ""

    # --- Cola --------------------------------------------------------------
    redis_url: str = "redis://redis:6379"
    queue_key: str = "tts:queue"

    # --- API interna -------------------------------------------------------
    host: str = "0.0.0.0"
    port: int = 7000
    api_key: str = ""
    # Clave que exige el backend Go. Si se deja vacia el servicio acepta
    # peticiones sin cabecera: solo es aceptable en la red interna de Docker.
    require_api_key: bool = True

    # --- Motor -------------------------------------------------------------
    device: str = "cpu"
    engines: List[str] = field(default_factory=lambda: ["chatterbox", "melotts", "openvoice"])
    default_engine: str = "chatterbox"
    melo_language: str = "ES"
    melo_speaker_id: str = "0"
    # Directorio que contiene checkpoints_v2/ para el motor OpenVoice.
    openvoice_model_dir: str = "/app/models/openvoice"
    # Embedding de la voz base que genera el audio (origen de la conversion).
    openvoice_source_embedding: str = ""
    # Manifiesto con las voces de las que hay embedding precalculado.
    openvoice_base_speakers_manifest: str = "/app/models/openvoice/base_speakers/manifest.json"

    # --- Chatterbox --------------------------------------------------------
    # Directorio con los pesos planos que lee ChatterboxMultilingualTTS.from_local
    # (ve.pt, s3gen.pt, t3_*.safetensors, conds.pt, grapheme_*.json).
    chatterbox_model_dir: str = "/app/models/chatterbox"
    # Peso del transformador multilingue: "v3" (recomendado), "v2" o el nombre
    # completo de un fichero .safetensors dentro del directorio de pesos.
    chatterbox_t3_model: str = "v3"
    # Idioma por defecto (codigo de 2 letras de Chatterbox) para las voces que
    # no declaren idioma propio.
    chatterbox_language: str = "es"
    # Controles expresivos del modelo. Los valores por defecto de Chatterbox
    # (0.5 / 0.5) son los recomendados para locucion neutra.
    chatterbox_exaggeration: float = 0.5
    chatterbox_cfg_weight: float = 0.5
    chatterbox_temperature: float = 0.8
    chatterbox_repetition_penalty: float = 1.2

    # Numero de hilos de torch. No se cambia solo: se fija por configuracion.
    torch_threads: int = 2

    # --- Almacenamiento -----------------------------------------------------
    audio_dir: str = "/app/data/tts_audio"
    voices_dir: str = "/app/data/tts_voices"
    tmp_dir: str = "/app/data/tts_tmp"

    # --- Voces -------------------------------------------------------------
    builtin_voices: List[str] = field(default_factory=lambda: ["es-base"])
    # Manifiesto con las voces clonadas preinstaladas (generado por
    # scripts/prepare_tts_models.sh). Si no existe, solo hay la voz base.
    builtin_voices_manifest: str = "/app/models/openvoice/voices/manifest.json"
    allow_voice_upload: bool = False

    # --- Trabajo -----------------------------------------------------------
    max_script_chars: int = 8000
    max_attempts: int = 2
    sleep_idle: float = 5.0
    # Cada cuanto revisa el vigilante las tareas atascadas (s).
    watchdog_interval: float = 30.0
    # Una tarea 'running' cuyo started_at es mas antiguo se considera zombie
    # y vuelve a la cola (recuperacion tras reinicio).
    job_timeout_seconds: int = 3600
    # Duracion maxima aceptada de una muestra de voz propia.
    min_sample_seconds: float = 3.0
    max_sample_seconds: float = 60.0
    # Velocidad de locucion (1.0 = normal).
    default_speed: float = 1.0

    # --- Normalizacion de guiones -----------------------------------------
    normalize_script: bool = True

    @property
    def db_config(self) -> dict:
        if self.db_url:
            return {"dsn": self.db_url}
        return {
            "host": self.db_host,
            "port": self.db_port,
            "dbname": self.db_name,
            "user": self.db_user,
            "password": self.db_pass,
        }

    @classmethod
    def from_env(cls) -> "TTSConfig":
        return cls(
            db_host=_env_str("DB_HOST", "db"),
            db_port=_env_int("DB_PORT", 5432),
            db_name=_env_str("DB_NAME", "rss"),
            db_user=_env_str("DB_USER", "rss"),
            db_pass=os.environ.get("DB_PASS", ""),
            db_url=_env_str("DATABASE_URL", ""),
            redis_url=_env_str("REDIS_URL", "redis://redis:6379"),
            queue_key=_env_str("TTS_QUEUE_KEY", "tts:queue"),
            host=_env_str("TTS_HOST", "0.0.0.0"),
            port=_env_int("TTS_PORT", 7000),
            api_key=_env_str("TTS_API_KEY", ""),
            require_api_key=_env_bool("TTS_REQUIRE_API_KEY", True),
            device=_env_str("TTS_DEVICE", "cpu"),
            engines=_env_list("TTS_ENGINES", "chatterbox,melotts,openvoice"),
            default_engine=_env_str("TTS_ENGINE", "chatterbox"),
            melo_language=_env_str("TTS_MELO_LANGUAGE", "ES"),
            melo_speaker_id=_env_str("TTS_MELO_SPEAKER_ID", "0"),
            openvoice_model_dir=_env_str("TTS_OPENVOICE_MODEL_DIR", "/app/models/openvoice"),
            openvoice_source_embedding=_env_str("TTS_OPENVOICE_SOURCE_EMBEDDING", ""),
            openvoice_base_speakers_manifest=_env_str(
                "TTS_OPENVOICE_BASE_SPEAKERS", "/app/models/openvoice/base_speakers/manifest.json"
            ),
            chatterbox_model_dir=_env_str("TTS_CHATTERBOX_MODEL_DIR", "/app/models/chatterbox"),
            chatterbox_t3_model=_env_str("TTS_CHATTERBOX_T3_MODEL", "v3"),
            chatterbox_language=_env_str("TTS_CHATTERBOX_LANGUAGE", "es"),
            chatterbox_exaggeration=_env_float("TTS_CHATTERBOX_EXAGGERATION", 0.5),
            chatterbox_cfg_weight=_env_float("TTS_CHATTERBOX_CFG_WEIGHT", 0.5),
            chatterbox_temperature=_env_float("TTS_CHATTERBOX_TEMPERATURE", 0.8),
            chatterbox_repetition_penalty=_env_float("TTS_CHATTERBOX_REPETITION_PENALTY", 1.2),
            torch_threads=_env_int("TTS_TORCH_THREADS", 2),
            audio_dir=_env_str("TTS_AUDIO_DIR", "/app/data/tts_audio"),
            voices_dir=_env_str("TTS_VOICES_DIR", "/app/data/tts_voices"),
            tmp_dir=_env_str("TTS_TMP_DIR", "/app/data/tts_tmp"),
            builtin_voices=_env_list("TTS_BUILTIN_VOICES", "es-base"),
            builtin_voices_manifest=_env_str(
                "TTS_BUILTIN_VOICES_MANIFEST", "/app/models/openvoice/voices/manifest.json"
            ),
            allow_voice_upload=_env_bool("TTS_ALLOW_VOICE_UPLOAD", False),
            max_script_chars=_env_int("TTS_MAX_SCRIPT_CHARS", 8000),
            max_attempts=_env_int("TTS_MAX_ATTEMPTS", 2),
            sleep_idle=_env_float("TTS_SLEEP_IDLE", 5.0),
            watchdog_interval=_env_float("TTS_WATCHDOG_INTERVAL", 30.0),
            job_timeout_seconds=_env_int("TTS_JOB_TIMEOUT_SECONDS", 3600),
            min_sample_seconds=_env_float("TTS_MIN_SAMPLE_SECONDS", 3.0),
            max_sample_seconds=_env_float("TTS_MAX_SAMPLE_SECONDS", 60.0),
            default_speed=_env_float("TTS_DEFAULT_SPEED", 1.0),
            normalize_script=_env_bool("TTS_NORMALIZE_SCRIPT", True),
        )
