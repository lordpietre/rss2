BEGIN;

-- Tabla eventos ya existe en 00-complete-schema.sql
-- Agregar columna evento_id a traducciones si no existe
ALTER TABLE traducciones
    ADD COLUMN IF NOT EXISTS evento_id BIGINT REFERENCES eventos(id);

-- Tabla eventos_noticias ya existe en 00-complete-schema.sql

-- Índices adicionales
CREATE INDEX IF NOT EXISTS idx_traducciones_evento
    ON traducciones(evento_id);

CREATE INDEX IF NOT EXISTS idx_traducciones_evento_fecha
    ON traducciones(evento_id, noticia_id);

CREATE INDEX IF NOT EXISTS idx_eventos_fecha
    ON eventos (fecha DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_eventos_noticias_evento
    ON eventos_noticias (evento_id);

CREATE INDEX IF NOT EXISTS idx_eventos_noticias_noticia
    ON eventos_noticias (noticia_id);

CREATE INDEX IF NOT EXISTS idx_eventos_noticias_traduccion
    ON eventos_noticias (traduccion_id);

COMMIT;

