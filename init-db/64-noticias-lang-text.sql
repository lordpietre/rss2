-- noticias.lang: CHAR(5) -> TEXT
--
-- CHAR(5) tiene dos problemas:
--   1) TRUNCA en silencio: un valor mas largo que 5 caracteres se recorta
--      ('english' -> 'engli') sin ningun error.
--   2) Rellena con espacios: el valor real es 'es   ', no 'es'. Las
--      comparaciones funcionan (Postgres hace rtrim al castear a text), pero
--      es una trampa y obliga a TRIM() en cada sitio.
--
-- TEXT con el TRIM inicial deja los valores limpios ('es', 'en', '').
-- Los indices sobre la columna se reconstruyen (el de lang y el parcial de
-- traduidas).
--
-- Es seguro con datos en curso: el languages se escribe siempre con codigos
-- ISO 639-1 de 2 letras (langdetect) o el idioma destino (ingestor).

ALTER TABLE noticias
    ALTER COLUMN lang TYPE TEXT USING NULLIF(TRIM(lang), '');

-- El indice sobre la columna se reconstruye con el nuevo tipo.
DROP INDEX IF EXISTS idx_noticias_lang;
CREATE INDEX idx_noticias_lang ON noticias (lang) WHERE lang IS NOT NULL AND lang <> '';

-- El indice parcial de "noticias traduidas" (canales ya en espanol) tambien
-- depende del tipo de la columna.
DROP INDEX IF EXISTS idx_noticias_trad_elegibles;
CREATE INDEX idx_noticias_trad_elegibles
    ON noticias (created_at DESC)
    WHERE lang IS NOT NULL AND lang <> 'es';
