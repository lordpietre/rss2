CREATE TABLE IF NOT EXISTS traducciones (
  id SERIAL PRIMARY KEY,
  noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
  lang_from CHAR(5),
  lang_to   CHAR(5) NOT NULL,
  titulo_trad   TEXT,
  resumen_trad  TEXT,
  status   VARCHAR(16) DEFAULT 'done',
  error    TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE (noticia_id, lang_to)
);

CREATE INDEX IF NOT EXISTS traducciones_to_idx ON traducciones (lang_to);

