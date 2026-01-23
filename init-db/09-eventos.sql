BEGIN;

CREATE TABLE IF NOT EXISTS eventos (
    id                  BIGSERIAL PRIMARY KEY,
    creado_en           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    titulo              TEXT,
    fecha_inicio        TIMESTAMPTZ,
    fecha_fin           TIMESTAMPTZ,
    n_noticias          INTEGER      NOT NULL DEFAULT 0,
    centroid            JSONB        NOT NULL,
    total_traducciones  INTEGER      NOT NULL DEFAULT 1
);

ALTER TABLE traducciones
    ADD COLUMN IF NOT EXISTS evento_id BIGINT REFERENCES eventos(id);

CREATE TABLE IF NOT EXISTS eventos_noticias (
    evento_id     BIGINT       NOT NULL REFERENCES eventos(id)      ON DELETE CASCADE,
    noticia_id    VARCHAR(32)  NOT NULL REFERENCES noticias(id)     ON DELETE CASCADE,
    traduccion_id INTEGER      NOT NULL REFERENCES traducciones(id) ON DELETE CASCADE,
    PRIMARY KEY (evento_id, noticia_id)
);

CREATE INDEX IF NOT EXISTS idx_traducciones_evento
    ON traducciones(evento_id);

CREATE INDEX IF NOT EXISTS idx_traducciones_evento_fecha
    ON traducciones(evento_id, noticia_id);

CREATE INDEX IF NOT EXISTS idx_trad_id
    ON traducciones(id);

CREATE INDEX IF NOT EXISTS idx_eventos_fecha_inicio
    ON eventos (fecha_inicio DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_eventos_noticias_evento
    ON eventos_noticias (evento_id);

CREATE INDEX IF NOT EXISTS idx_eventos_noticias_noticia
    ON eventos_noticias (noticia_id);

CREATE INDEX IF NOT EXISTS idx_eventos_noticias_traduccion
    ON eventos_noticias (traduccion_id);

CREATE OR REPLACE FUNCTION actualizar_evento_modificado()
RETURNS TRIGGER AS $$
BEGIN
    NEW.actualizado_en = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_evento_modificado ON eventos;

CREATE TRIGGER trg_evento_modificado
BEFORE UPDATE ON eventos
FOR EACH ROW
EXECUTE FUNCTION actualizar_evento_modificado();

COMMIT;

