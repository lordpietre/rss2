-- init-db/06-tags.sql (compatible with existing schema from 00-complete-schema.sql)

-- Tabla de tags ya existe en 00-complete-schema.sql con columnas 'valor' y 'tipo'
-- Solo crear índices adicionales si no existen

-- Índices útiles
CREATE INDEX IF NOT EXISTS idx_tags_valor ON tags(valor);
CREATE INDEX IF NOT EXISTS idx_tags_tipo ON tags(tipo);
CREATE INDEX IF NOT EXISTS idx_tags_noticia_trid ON tags_noticia(traduccion_id);
CREATE INDEX IF NOT EXISTS idx_tags_noticia_tag  ON tags_noticia(tag_id);

-- Wikipedia data columns
ALTER TABLE tags ADD COLUMN IF NOT EXISTS wiki_summary TEXT;
ALTER TABLE tags ADD COLUMN IF NOT EXISTS wiki_url TEXT;
ALTER TABLE tags ADD COLUMN IF NOT EXISTS image_path TEXT;
ALTER TABLE tags ADD COLUMN IF NOT EXISTS wiki_checked BOOLEAN DEFAULT FALSE;
ALTER TABLE tags ADD COLUMN IF NOT EXISTS wiki_checked BOOLEAN DEFAULT FALSE;
