-- Embeddings del TEXTO ORIGINAL de noticias que ya estan en el idioma destino.
--
-- Las noticias de los canales marcados como 'traducido' (y las que langdetect
-- detecta como espanol) no generan fila en traducciones: el scheduler no
-- encola traducciones para ellas. Pero su texto original ya esta en espanol y
-- debe poder aparecer en la busqueda semantica igual que las demas.
--
-- Se reutiliza la tabla traduccion_embeddings con traduccion_id NULL y
-- noticia_id informado (la FK es nullable). El indice unico parcial evita
-- duplicar embeddings del mismo texto original.
ALTER TABLE traduccion_embeddings
    ADD COLUMN IF NOT EXISTS noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE;

-- Un embedding por noticia y modelo (traduccion_id NULL no es clave valida
-- para la constraint unica existente).
CREATE UNIQUE INDEX IF NOT EXISTS idx_tr_emb_noticia_model
    ON traduccion_embeddings (noticia_id, model)
    WHERE noticia_id IS NOT NULL;

-- Indice para el anti-join del worker de embeddings.
CREATE INDEX IF NOT EXISTS idx_tr_emb_noticia_lookup
    ON traduccion_embeddings (noticia_id, model)
    WHERE noticia_id IS NOT NULL AND traduccion_id IS NULL;
