-- Optimización de índices
CREATE INDEX IF NOT EXISTS idx_noticias_pais ON noticias(pais_id);
CREATE INDEX IF NOT EXISTS idx_noticias_categoria ON noticias(categoria_id);
CREATE INDEX IF NOT EXISTS idx_traducciones_created_at ON traducciones(created_at);
CREATE INDEX IF NOT EXISTS idx_news_topics_topic_id ON news_topics(topic_id);
