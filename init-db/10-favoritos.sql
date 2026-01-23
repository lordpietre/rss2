-- Favorites table for saving news
CREATE TABLE IF NOT EXISTS favoritos (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL,
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (session_id, noticia_id)
);

CREATE INDEX IF NOT EXISTS idx_favoritos_session ON favoritos(session_id);
