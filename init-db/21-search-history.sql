-- Tabla de historial de búsquedas
-- Registra todas las búsquedas realizadas por usuarios autenticados

CREATE TABLE IF NOT EXISTS search_history (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    results_count INTEGER DEFAULT 0,
    searched_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT query_not_empty CHECK (LENGTH(TRIM(query)) > 0)
);

-- Índices para queries eficientes
CREATE INDEX IF NOT EXISTS idx_search_history_user_date 
    ON search_history(user_id, searched_at DESC);
CREATE INDEX IF NOT EXISTS idx_search_history_user_id 
    ON search_history(user_id);

-- Índice para buscar búsquedas populares
CREATE INDEX IF NOT EXISTS idx_search_history_query 
    ON search_history(query);

-- Comentarios
COMMENT ON TABLE search_history IS 'Historial de búsquedas de usuarios';
COMMENT ON COLUMN search_history.user_id IS 'Usuario que realizó la búsqueda';
COMMENT ON COLUMN search_history.query IS 'Término de búsqueda';
COMMENT ON COLUMN search_history.results_count IS 'Cantidad de resultados encontrados';
