-- migrations/add_tts.sql
-- Equivalente de init-db/57-add-tts.sql para bases ya existentes.
-- Aplicar con:
--   docker compose exec -T db psql -U rss -d rss < migrations/add_tts.sql
-- (el servicio `tts` tambien asegura las tablas al arrancar).


-- -----------------------------------------------------------------------------
-- Voces: perfiles de timbre que puede usar el motor.
-- engine = 'melotts'  -> sintesis estandar con la voz base del idioma
-- engine = 'openvoice' -> conversion de timbre usando un audio de referencia
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tts_voices (
    id               SERIAL PRIMARY KEY,
    slug             VARCHAR(64) NOT NULL UNIQUE,
    display_name     VARCHAR(128) NOT NULL,
    engine           VARCHAR(32) NOT NULL DEFAULT 'melotts',
    language         CHAR(5) NOT NULL DEFAULT 'es',
    speaker_id       VARCHAR(16),
    reference_file   TEXT,
    duration_seconds DOUBLE PRECISION,
    sample_rate      INTEGER,
    quality_status   VARCHAR(16) NOT NULL DEFAULT 'unchecked',
    quality_report   JSONB,
    is_builtin       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMP DEFAULT NOW(),
    updated_at       TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tts_voices_engine ON tts_voices(engine);

-- -----------------------------------------------------------------------------
-- Guiones: texto editorial aprobado para locucion.
-- raw_content guarda el texto tal cual venia de la noticia; content guarda la
-- version normalizada que realmente se locuta. Se conserva la referencia a la
-- fuente original (noticia / traduccion / url / medio).
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tts_scripts (
    id              SERIAL PRIMARY KEY,
    noticia_id      VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    translation_id  INTEGER REFERENCES traducciones(id) ON DELETE SET NULL,
    language        CHAR(5) NOT NULL DEFAULT 'es',
    title           TEXT,
    raw_content     TEXT,
    content         TEXT NOT NULL,
    source_title    TEXT,
    source_url      TEXT,
    source_name     TEXT,
    status          VARCHAR(16) NOT NULL DEFAULT 'draft',
    created_by      INTEGER REFERENCES users(id) ON DELETE SET NULL,
    approved_by     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    approved_at     TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tts_scripts_noticia ON tts_scripts(noticia_id);
CREATE INDEX IF NOT EXISTS idx_tts_scripts_status ON tts_scripts(status);
CREATE INDEX IF NOT EXISTS idx_tts_scripts_created_at ON tts_scripts(created_at DESC);

-- -----------------------------------------------------------------------------
-- Tareas de sintesis. Cola persistente: si el worker muere, la tarea sigue en
-- 'queued' y se recupera al reiniciar el servicio.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tts_jobs (
    id               BIGSERIAL PRIMARY KEY,
    script_id        INTEGER NOT NULL REFERENCES tts_scripts(id) ON DELETE CASCADE,
    voice_id         INTEGER REFERENCES tts_voices(id) ON DELETE SET NULL,
    engine           VARCHAR(32) NOT NULL DEFAULT 'melotts',
    status           VARCHAR(16) NOT NULL DEFAULT 'queued',
    idempotency_key  VARCHAR(160) UNIQUE,
    speed            REAL NOT NULL DEFAULT 1.0,
    attempts         INTEGER NOT NULL DEFAULT 0,
    max_attempts     INTEGER NOT NULL DEFAULT 2,
    error_code       VARCHAR(64),
    error_message    TEXT,
    queued_at        TIMESTAMP DEFAULT NOW(),
    started_at       TIMESTAMP,
    finished_at      TIMESTAMP,
    created_at       TIMESTAMP DEFAULT NOW(),
    updated_at       TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tts_jobs_status ON tts_jobs(status);
CREATE INDEX IF NOT EXISTS idx_tts_jobs_script ON tts_jobs(script_id);

-- -----------------------------------------------------------------------------
-- Audios generados. status = pending_review hasta que un editor lo aprueba.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tts_audios (
    id                SERIAL PRIMARY KEY,
    job_id            BIGINT NOT NULL REFERENCES tts_jobs(id) ON DELETE CASCADE,
    script_id         INTEGER NOT NULL REFERENCES tts_scripts(id) ON DELETE CASCADE,
    voice_id          INTEGER REFERENCES tts_voices(id) ON DELETE SET NULL,
    engine            VARCHAR(32) NOT NULL,
    engine_version    VARCHAR(64),
    storage_path      TEXT NOT NULL,
    format            VARCHAR(8) NOT NULL DEFAULT 'wav',
    duration_seconds  DOUBLE PRECISION,
    sample_rate       INTEGER,
    size_bytes        BIGINT,
    sha256            CHAR(64),
    parameters        JSONB NOT NULL DEFAULT '{}'::jsonb,
    status            VARCHAR(16) NOT NULL DEFAULT 'pending_review',
    approved_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    approved_at       TIMESTAMP,
    created_at        TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tts_audios_job ON tts_audios(job_id);
CREATE INDEX IF NOT EXISTS idx_tts_audios_status ON tts_audios(status);
CREATE INDEX IF NOT EXISTS idx_tts_audios_script ON tts_audios(script_id);

-- -----------------------------------------------------------------------------
-- Noticiario: una edicion ordenada de varias narraciones, concatenada en un
-- unico archivo de audio. Cada item conserva su guion y su noticia de origen.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tts_bulletins (
    id               SERIAL PRIMARY KEY,
    title            TEXT NOT NULL,
    language         CHAR(5) NOT NULL DEFAULT 'es',
    status           VARCHAR(16) NOT NULL DEFAULT 'pending',
    voice_id         INTEGER REFERENCES tts_voices(id) ON DELETE SET NULL,
    engine           VARCHAR(32) NOT NULL DEFAULT 'melotts',
    speed            REAL NOT NULL DEFAULT 1.0,
    audio_path       TEXT,
    duration_seconds DOUBLE PRECISION,
    size_bytes       BIGINT,
    sha256           CHAR(64),
    item_count       INTEGER NOT NULL DEFAULT 0,
    silence_ms       INTEGER NOT NULL DEFAULT 800,
    created_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    approved_by      INTEGER REFERENCES users(id) ON DELETE SET NULL,
    approved_at      TIMESTAMP,
    created_at       TIMESTAMP DEFAULT NOW(),
    updated_at       TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tts_bulletins_status ON tts_bulletins(status);
CREATE INDEX IF NOT EXISTS idx_tts_bulletins_created_at ON tts_bulletins(created_at DESC);

CREATE TABLE IF NOT EXISTS tts_bulletin_items (
    id               SERIAL PRIMARY KEY,
    bulletin_id      INTEGER NOT NULL REFERENCES tts_bulletins(id) ON DELETE CASCADE,
    script_id        INTEGER NOT NULL REFERENCES tts_scripts(id) ON DELETE CASCADE,
    position         INTEGER NOT NULL,
    audio_id         INTEGER REFERENCES tts_audios(id) ON DELETE SET NULL,
    audio_path       TEXT,
    duration_seconds DOUBLE PRECISION,
    start_offset     DOUBLE PRECISION,
    source_url       TEXT,
    source_name      TEXT,
    source_title     TEXT,
    created_at       TIMESTAMP DEFAULT NOW(),
    UNIQUE (bulletin_id, position),
    UNIQUE (bulletin_id, script_id)
);

CREATE INDEX IF NOT EXISTS idx_tts_bulletin_items_bulletin ON tts_bulletin_items(bulletin_id);
CREATE INDEX IF NOT EXISTS idx_tts_bulletin_items_script ON tts_bulletin_items(script_id);

-- -----------------------------------------------------------------------------
-- Compatibilidad: las tablas pueden existir de una version anterior.
-- -----------------------------------------------------------------------------
ALTER TABLE tts_jobs ADD COLUMN IF NOT EXISTS speed REAL NOT NULL DEFAULT 1.0;
ALTER TABLE tts_bulletins ADD COLUMN IF NOT EXISTS voice_id INTEGER REFERENCES tts_voices(id) ON DELETE SET NULL;
ALTER TABLE tts_bulletins ADD COLUMN IF NOT EXISTS engine VARCHAR(32) NOT NULL DEFAULT 'melotts';
ALTER TABLE tts_bulletins ADD COLUMN IF NOT EXISTS speed REAL NOT NULL DEFAULT 1.0;
