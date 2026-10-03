-- migrations/add_clean_columns.sql
-- Equivalente de init-db/45-limpieza-textos.sql para bases ya existentes.
-- Aplicar con:  docker compose exec -T db psql -U rss -d rss < migrations/add_clean_columns.sql
-- (el binario `sanitize` también las crea si faltan).

ALTER TABLE noticias ADD COLUMN IF NOT EXISTS titulo_raw TEXT;
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS resumen_raw TEXT;
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS cleaned_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_noticias_cleaned_null
    ON noticias (id) WHERE cleaned_at IS NULL;

COMMENT ON COLUMN noticias.resumen_raw IS 'Resumen original antes de textclean (NULL si no cambió)';
COMMENT ON COLUMN noticias.titulo_raw  IS 'Título original antes de textclean (NULL si no cambió)';
COMMENT ON COLUMN noticias.cleaned_at  IS 'Última pasada del sanitizador sobre esta fila';
