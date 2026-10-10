-- Índice para optimizar consultas de entity news
-- El JOIN más común es: tags_noticia tn JOIN tags t ON tn.tag_id = t.id WHERE tn.noticia_id = $1
-- Sin este índice, Postgres hace un sequential scan en tags_noticia para cada noticia
CREATE INDEX IF NOT EXISTS idx_tags_noticia_noticia_id ON tags_noticia(noticia_id);
