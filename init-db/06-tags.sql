-- init-db/06-tags.sql  (modelo simple compatible con ner_worker.py)

-- Tabla de tags
CREATE TABLE IF NOT EXISTS tags (
  id    SERIAL PRIMARY KEY,
  valor TEXT NOT NULL,
  tipo  TEXT NOT NULL,         -- 'persona','organizacion','lugar', ...
  UNIQUE (valor, tipo)
);

-- Relación tag <-> traducción
CREATE TABLE IF NOT EXISTS tags_noticia (
  id             SERIAL PRIMARY KEY,
  traduccion_id  INT NOT NULL REFERENCES traducciones(id) ON DELETE CASCADE,
  tag_id         INT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  UNIQUE (traduccion_id, tag_id)
);

-- Índices útiles
CREATE INDEX IF NOT EXISTS idx_tags_valor ON tags(valor);
CREATE INDEX IF NOT EXISTS idx_tags_tipo  ON tags(tipo);
CREATE INDEX IF NOT EXISTS idx_tags_noticia_trid ON tags_noticia(traduccion_id);
CREATE INDEX IF NOT EXISTS idx_tags_noticia_tag  ON tags_noticia(tag_id);

