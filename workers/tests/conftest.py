"""Configuracion comun de las pruebas del subsistema TTS.

Las pruebas unitarias no descargan pesos ni arrancan el motor: todo lo que
necesita el modelo se sustituye por dobles de prueba.

Las pruebas de integracion (las que necesitan Postgres) se activan definiendo
TEST_DATABASE_URL; si no esta definida se saltan. Para crearla:

    make test-tts-db

que crea la base `rss_tts_test` con las tablas minimas y el esquema TTS.
"""

from __future__ import annotations

import os
import sys

import pytest

# La raiz del repositorio, para poder importar "workers.tts...".
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
if REPO_ROOT not in sys.path:
    sys.path.insert(0, REPO_ROOT)

TEST_DATABASE_URL = os.environ.get("TEST_DATABASE_URL", "")

# Tablas minimas que necesitan las claves foraneas del subsistema TTS.
PREREQ_SQL = [
    """
    CREATE TABLE IF NOT EXISTS users (
        id    SERIAL PRIMARY KEY,
        email VARCHAR(255)
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS noticias (
        id  VARCHAR(32) PRIMARY KEY,
        url TEXT
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS traducciones (
        id            SERIAL PRIMARY KEY,
        noticia_id    VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
        lang_from     CHAR(5),
        lang_to       CHAR(5),
        titulo_trad   TEXT,
        resumen_trad  TEXT,
        contenido_trad TEXT,
        status        VARCHAR(16) DEFAULT 'done'
    )
    """,
]


def _database_available() -> bool:
    if not TEST_DATABASE_URL:
        return False
    try:
        import psycopg2

        conn = psycopg2.connect(TEST_DATABASE_URL, connect_timeout=3)
        conn.close()
        return True
    except Exception:
        return False


DB_AVAILABLE = _database_available()

requires_db = pytest.mark.skipif(
    not DB_AVAILABLE,
    reason="Requiere TEST_DATABASE_URL apuntando a un Postgres de pruebas",
)


@pytest.fixture(scope="session")
def db_url() -> str:
    if not DB_AVAILABLE:
        pytest.skip("Postgres de pruebas no disponible")
    return TEST_DATABASE_URL


@pytest.fixture
def db_conn(db_url):
    """Conexion a Postgres de pruebas.

    Se prepara el esquema minimo y, al terminar, se vacian las tablas del
    subsistema TTS para que las pruebas no se contaminen entre si.
    """
    import psycopg2

    from workers.tts.schema import ensure_schema

    conn = psycopg2.connect(db_url)
    try:
        with conn.cursor() as cur:
            for statement in PREREQ_SQL:
                cur.execute(statement)
        conn.commit()
        ensure_schema(conn)

        yield conn

        with conn.cursor() as cur:
            cur.execute(
                "TRUNCATE tts_audios, tts_jobs, tts_scripts, tts_voices, "
                "traducciones, noticias RESTART IDENTITY CASCADE"
            )
        conn.commit()
    finally:
        conn.rollback()
        conn.close()


@pytest.fixture
def tmp_dirs(tmp_path):
    """Directorios temporales de audio, voces y trabajo."""
    audio = tmp_path / "audio"
    voices = tmp_path / "voices"
    work = tmp_path / "tmp"
    for directory in (audio, voices, work):
        directory.mkdir(parents=True, exist_ok=True)
    return {"audio": str(audio), "voices": str(voices), "tmp": str(work), "root": str(tmp_path)}


@pytest.fixture
def tts_config(db_url, tmp_dirs):
    """Configuracion del servicio apuntando a la base y directorios de test."""
    from workers.tts.config import TTSConfig

    cfg = TTSConfig()
    cfg.db_url = db_url
    cfg.audio_dir = tmp_dirs["audio"]
    cfg.voices_dir = tmp_dirs["voices"]
    cfg.tmp_dir = tmp_dirs["tmp"]
    cfg.engines = ["fake"]
    cfg.default_engine = "fake"
    cfg.builtin_voices = ["es-base"]
    cfg.sleep_idle = 0.05
    cfg.redis_url = "redis://127.0.0.1:1"  # no disponible: se usa la cola de Postgres
    return cfg


@pytest.fixture
def service(tts_config, db_conn):
    """Servicio de TTS con motor de prueba y base de datos de test."""
    import numpy as np
    import soundfile

    from workers.tts.engine import SynthesisResult, VoiceEngine
    from workers.tts.service import TTSService

    class FakeEngine(VoiceEngine):
        name = "fake"
        version = "fake-1"

        def __init__(self, cfg, failure=None):
            super().__init__(cfg)
            self.failure = failure
            self.calls = []

        def synthesize(self, text, voice, output_path, params=None):
            self.calls.append(text)
            if self.failure is not None:
                raise self.failure
            samples = np.zeros(11025, dtype=np.float32)
            soundfile.write(output_path, samples, 22050)
            return SynthesisResult(
                path=output_path,
                duration_seconds=0.5,
                sample_rate=22050,
                engine=self.name,
                engine_version=self.version,
                parameters=params or {},
            )

    svc = TTSService(tts_config)
    svc.conn = db_conn
    svc.engines["fake"] = FakeEngine(tts_config)
    svc.FakeEngine = FakeEngine
    # La voz base del motor (no requiere audio de referencia).
    svc._seed_builtin_voices()
    return svc
