-- Seguimiento de los intentos de enriquecimiento (scraper).
--
-- El scraper seleccionaba `WHERE contenido IS NULL ORDER BY fecha DESC LIMIT 20`.
-- Si la extraccion fallaba (404, 403, URL relativa), la noticia se quedaba con
-- contenido NULL y volvia a ser seleccionada en el siguiente ciclo: las mismas
-- 20 noticias se reintentaban indefinidamente y el enriquecimiento nunca
-- avanzaba (0 noticias enriquecidas el ultimo dia) mientras el worker quemaba
-- CPU y llenaba el log de errores.
--
-- Con este seguimiento el scraper cuenta los intentos y salta durante una
-- semana lo que ya fallo, de modo que la cola sigue avanzando.
--
-- Va en una tabla aparte y no como columnas de 'noticias' a proposito: un
-- UPDATE sobre 'noticias' dispara el trigger update_noticias_updated_at, que
-- refresca updated_at. La ruta de catch-up del translation_scheduler busca
-- precisamente las noticias actualizadas en las ultimas horas, asi que
-- contabilizar los intentos desde 'noticias' generaria miles de trabajos de
-- traduccion que no corresponden.

CREATE TABLE IF NOT EXISTS noticias_enrich_log (
    noticia_id   VARCHAR(32) PRIMARY KEY REFERENCES noticias(id) ON DELETE CASCADE,
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_attempt TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error   TEXT
);

-- Indice parcial para localizar las noticias que aun no tienen contenido.
-- Es MUCHO mas pequeno que el indice general por created_at (que el ingestor
-- y el scheduler usan para otras cosas).
CREATE INDEX IF NOT EXISTS idx_noticias_enrich_pending
    ON noticias (created_at DESC)
    WHERE contenido IS NULL;

COMMENT ON TABLE noticias_enrich_log IS
    'Intentos fallidos de enriquecimiento (scraper). A los 3 la noticia se salta una semana.';
COMMENT ON COLUMN noticias_enrich_log.attempts IS
    'Numero de intentos fallidos de enriquecimiento.';
COMMENT ON COLUMN noticias_enrich_log.last_attempt IS
    'Ultimo intento de enriquecimiento, para el periodo de enfriamiento.';
COMMENT ON COLUMN noticias_enrich_log.last_error IS
    'Ultimo error de enriquecimiento (diagnostico).';
