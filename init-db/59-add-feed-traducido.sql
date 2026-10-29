-- Canales que ya estan en el idioma destino (espanol) y por tanto no
-- necesitan traduccion.
--
-- Las noticias de estos canales se insertan directamente con lang='es', de
-- modo que el pipeline las salta:
--   - langdetect solo procesa noticias con lang NULL/vacio -> no las detecta
--   - el scheduler excluye n.lang = 'es' -> no genera job de traduccion
-- Se ahorra la CPU de detectar el idioma y de traducir.
--
-- Es una lista curada a mano (no se deduce de feeds.idioma, que es metadato
-- declarado por el feed y a veces esta equivocado).
ALTER TABLE feeds
    ADD COLUMN IF NOT EXISTS traducido BOOLEAN NOT NULL DEFAULT FALSE;

-- Indice parcial: la lista de canales "traducidos" se consulta y se filtra
-- a menudo, pero son una minoria.
CREATE INDEX IF NOT EXISTS idx_feeds_traducido
    ON feeds (nombre)
    WHERE traducido = TRUE;
