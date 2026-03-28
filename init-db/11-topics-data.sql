-- Tablas topics y news_topics ya existen en 00-complete-schema.sql con columnas: nombre, descripcion, weight, keywords
-- Agregar índice adicional

CREATE INDEX IF NOT EXISTS idx_news_topics_confidence ON news_topics(confidence DESC);
