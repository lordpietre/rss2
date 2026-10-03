-- 45-limpieza-textos.sql
-- Columnas que usa el binario `sanitize` (backend/cmd/sanitize) para limpiar
-- (backfill) los resúmenes/títulos ya existentes y para poder deshacerlo.
--
-- Idempotente: se puede ejecutar tantas veces como se quiera. El propio
-- binario también las crea si faltan, así que esta migración es opcional pero
-- recomendable antes de desplegar.
--
--   titulo_raw / resumen_raw : texto original, solo cuando la limpieza cambió
--                              algo (NULL = no hay nada que restaurar).
--   cleaned_at               : momento en que la fila fue revisada; el
--                              backfill solo mira cleaned_at IS NULL.

ALTER TABLE noticias ADD COLUMN IF NOT EXISTS titulo_raw TEXT;
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS resumen_raw TEXT;
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS cleaned_at TIMESTAMP;

-- El backfill recorre `WHERE cleaned_at IS NULL AND id > $1 ORDER BY id`;
-- el índice parcial lo mantiene barato también cuando quedan pocas filas.
CREATE INDEX IF NOT EXISTS idx_noticias_cleaned_null
    ON noticias (id) WHERE cleaned_at IS NULL;

COMMENT ON COLUMN noticias.resumen_raw IS 'Resumen original antes de textclean (NULL si no cambió)';
COMMENT ON COLUMN noticias.titulo_raw  IS 'Título original antes de textclean (NULL si no cambió)';
COMMENT ON COLUMN noticias.cleaned_at  IS 'Última pasada del sanitizador sobre esta fila';
