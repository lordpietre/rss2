-- =============================================================================
-- RSS2 - Script de inicialización completa de base de datos
-- Este script crea todas las tablas y columnas necesarias para el sistema
-- IMPORTANTE: Las tablas deben crearse en orden correcto (sin referencias a tablas que no existen)
-- Se ejecuta automáticamente al iniciar PostgreSQL (directorio init-db)
-- =============================================================================

-- =============================================================================
-- SECCIÓN 1: TABLAS BASE (sin foreign keys a otras tablas del schema)
-- =============================================================================

-- Tabla de usuarios (Auth)
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    is_admin BOOLEAN DEFAULT FALSE,
    role VARCHAR(20) DEFAULT 'user',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Tabla de configuración (Sistema)
CREATE TABLE IF NOT EXISTS config (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT NOW()
);
-- Insertar configuración por defecto si no existe
INSERT INTO config (key, value) VALUES ('translator_type', 'cpu') ON CONFLICT (key) DO NOTHING;
INSERT INTO config (key, value) VALUES ('translator_workers', '2') ON CONFLICT (key) DO NOTHING;
INSERT INTO config (key, value) VALUES ('translator_status', 'stopped') ON CONFLICT (key) DO NOTHING;

-- Tablas básicas
CREATE TABLE IF NOT EXISTS continentes (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS categorias (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS paises (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    continente_id INTEGER REFERENCES continentes(id) ON DELETE SET NULL
);

-- Tabla de feeds
CREATE TABLE IF NOT EXISTS feeds (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(255),
    descripcion TEXT,
    url TEXT NOT NULL UNIQUE,
    categoria_id INTEGER REFERENCES categorias(id) ON DELETE SET NULL,
    pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL,
    idioma CHAR(2),
    activo BOOLEAN DEFAULT TRUE,
    fallos INTEGER DEFAULT 0,
    last_etag TEXT,
    last_modified TEXT,
    last_error TEXT,
    last_fetch TIMESTAMP
);

-- Tabla de fuentes URL
CREATE TABLE IF NOT EXISTS fuentes_url (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(255) NOT NULL,
    url TEXT NOT NULL UNIQUE,
    categoria_id INTEGER REFERENCES categorias(id) ON DELETE SET NULL,
    pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL,
    idioma CHAR(2) DEFAULT 'es',
    last_check TIMESTAMP,
    last_status VARCHAR(50),
    status_message TEXT,
    last_http_code INTEGER,
    active BOOLEAN DEFAULT TRUE
);

-- Tabla de noticias
CREATE TABLE IF NOT EXISTS noticias (
    id VARCHAR(32) PRIMARY KEY,
    titulo TEXT,
    resumen TEXT,
    url TEXT NOT NULL UNIQUE,
    fecha TIMESTAMP,
    imagen_url TEXT,
    fuente_nombre VARCHAR(255),
    categoria_id INTEGER REFERENCES categorias(id) ON DELETE SET NULL,
    pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL,
    tsv tsvector,
    topics_processed BOOLEAN DEFAULT FALSE,
    lang CHAR(5)
);

-- Agregar columna lang si no existe
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS lang CHAR(5);

-- Trigger para búsqueda full-text en noticias
CREATE OR REPLACE FUNCTION noticias_tsv_trigger() RETURNS trigger AS $$
BEGIN
  new.tsv := setweight(to_tsvector('spanish', coalesce(new.titulo,'')), 'A') ||
             setweight(to_tsvector('spanish', coalesce(new.resumen,'')), 'B');
  return new;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tsvectorupdate ON noticias;
CREATE TRIGGER tsvectorupdate
BEFORE INSERT OR UPDATE ON noticias
FOR EACH ROW EXECUTE PROCEDURE noticias_tsv_trigger();

CREATE INDEX IF NOT EXISTS noticias_tsv_idx ON noticias USING gin(tsv);

-- Tabla de eventos (CRÍTICO: debe crearse ANTES de traducciones)
CREATE TABLE IF NOT EXISTS eventos (
    id SERIAL PRIMARY KEY,
    titulo VARCHAR(255),
    descripcion TEXT,
    fecha TIMESTAMP,
    fuente VARCHAR(100),
    url TEXT,
    idioma CHAR(2),
    pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    valor VARCHAR(255) NOT NULL,
    tipo VARCHAR(32) NOT NULL,
    UNIQUE(valor, tipo)
);

-- Tabla de topics
CREATE TABLE IF NOT EXISTS topics (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL UNIQUE,
    descripcion TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    weight REAL,
    keywords TEXT
);

-- Agregar columnas faltantes si no existen
ALTER TABLE topics ADD COLUMN IF NOT EXISTS weight REAL;
ALTER TABLE topics ADD COLUMN IF NOT EXISTS keywords TEXT;

-- =============================================================================
-- SECCIÓN 2: TABLAS DEPENDIENTES (con foreign keys a tablas de arriba)
-- =============================================================================

-- Tabla de traducciones (referencia noticias Y eventos)
CREATE TABLE IF NOT EXISTS traducciones (
    id SERIAL PRIMARY KEY,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    lang_from CHAR(5),
    lang_to CHAR(5) NOT NULL,
    titulo_trad TEXT,
    resumen_trad TEXT,
    status VARCHAR(16) DEFAULT 'done',
    error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    evento_id BIGINT REFERENCES eventos(id),
    locked_at TIMESTAMP,
    vectorized BOOLEAN DEFAULT FALSE,
    UNIQUE (noticia_id, lang_to)
);

-- Agregar columnas faltantes si no existen
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP;
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS vectorized BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_traducciones_noticia_lang_status ON traducciones(noticia_id, lang_to, status);
CREATE INDEX IF NOT EXISTS idx_traducciones_created_at ON traducciones(created_at);

-- Tabla pivote eventos_noticias
CREATE TABLE IF NOT EXISTS eventos_noticias (
    id SERIAL PRIMARY KEY,
    evento_id INTEGER REFERENCES eventos(id) ON DELETE CASCADE,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de favoritos
CREATE TABLE IF NOT EXISTS favoritos (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, noticia_id)
);

-- Tabla de search history
CREATE TABLE IF NOT EXISTS search_history (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    query VARCHAR(255),
    category_id INTEGER,
    country_id INTEGER,
    results_count INTEGER DEFAULT 0,
    searched_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de news_topics
CREATE TABLE IF NOT EXISTS news_topics (
    id SERIAL PRIMARY KEY,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    topic_id INTEGER REFERENCES topics(id) ON DELETE CASCADE,
    confidence REAL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(noticia_id, topic_id)
);

-- Tabla de related_noticias
CREATE TABLE IF NOT EXISTS related_noticias (
    id SERIAL PRIMARY KEY,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    related_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE,
    score REAL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de tags_noticia
CREATE TABLE IF NOT EXISTS tags_noticia (
    id SERIAL PRIMARY KEY,
    tag_id INTEGER REFERENCES tags(id) ON DELETE CASCADE,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tag_id, noticia_id)
);

-- =============================================================================
-- SECCIÓN 3: OTRAS TABLAS
-- =============================================================================

-- Tabla de entity aliases
CREATE TABLE IF NOT EXISTS entity_aliases (
    id SERIAL PRIMARY KEY,
    canonical_name VARCHAR(255) NOT NULL,
    alias VARCHAR(255) NOT NULL,
    tipo VARCHAR(50) NOT NULL CHECK (tipo IN ('persona', 'organizacion', 'lugar', 'tema')),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(alias, tipo)
);

-- Tabla de traduccion_embeddings
CREATE TABLE IF NOT EXISTS traduccion_embeddings (
    id SERIAL PRIMARY KEY,
    traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    dim INT NOT NULL,
    embedding DOUBLE PRECISION[] NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (traduccion_id, model)
);

-- Tabla de videos
CREATE TABLE IF NOT EXISTS videos (
    id SERIAL PRIMARY KEY,
    titulo VARCHAR(255),
    descripcion TEXT,
    url TEXT NOT NULL UNIQUE,
    thumbnail_url TEXT,
    duration INTEGER,
    published_at TIMESTAMP,
    fuente VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de video_parrillas
CREATE TABLE IF NOT EXISTS video_parrillas (
    id SERIAL PRIMARY KEY,
    titulo VARCHAR(255) NOT NULL,
    descripcion TEXT,
    fecha_inicio TIMESTAMP,
    fecha_fin TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- =============================================================================
-- NOTIFICAR QUE LA INICIALIZACIÓN ESTÁ COMPLETA
-- =============================================================================
DO $$
BEGIN
    RAISE NOTICE 'RSS2 Base de datos inicializada correctamente';
END $$;
