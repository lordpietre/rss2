-- =============================================================================
-- RSS2 - Limpieza de Noticias y Traducciones
-- Para empezar de cero con el proceso de traducción
-- =============================================================================

-- Desactivar triggers para speed
SET session_replication_role = 'replica';

-- Tablas que dependen de noticias (orden inverso por FK)
TRUNCATE TABLE related_noticias RESTART IDENTITY CASCADE;
TRUNCATE TABLE tags_noticia RESTART IDENTITY CASCADE;
TRUNCATE TABLE eventos_noticias RESTART IDENTITY CASCADE;
TRUNCATE TABLE news_topics RESTART IDENTITY CASCADE;

-- Tablas que dependen de traducciones
TRUNCATE TABLE traduccion_embeddings RESTART IDENTITY CASCADE;

-- Tablas principales
TRUNCATE TABLE noticias RESTART IDENTITY CASCADE;
TRUNCATE TABLE traducciones RESTART IDENTITY CASCADE;

-- Reactivar triggers
SET session_replication_role = 'origin';

-- Reset sequence de noticias
SELECT setval('noticias_id_seq', 1, false);

-- Reset sequence de traducciones  
SELECT setval('traducciones_id_seq', 1, false);

-- Reset sequences de tablas dependientes
SELECT setval('related_noticias_id_seq', 1, false);
SELECT setval('tags_noticia_id_seq', 1, false);
SELECT setval('eventos_noticias_id_seq', 1, false);
SELECT setval('news_topics_id_seq', 1, false);
SELECT setval('traduccion_embeddings_id_seq', 1, false);

-- Limpiar métricas si existe
TRUNCATE TABLE translation_stats RESTART IDENTITY CASCADE;

-- Limpiar favoritos y listas de usuarios (opcional - comenta si quieres mantener)
-- TRUNCATE TABLE favoritos RESTART IDENTITY CASCADE;
-- TRUNCATE TABLE user_favorites RESTART IDENTITY CASCADE;
-- TRUNCATE TABLE user_list_items RESTART IDENTITY CASCADE;

-- Limpiar alerts
TRUNCATE TABLE alertas RESTART IDENTITY CASCADE;

-- Limpiar search history
TRUNCATE TABLE search_history RESTART IDENTITY CASCADE;

DO $$
DECLARE
    news_count INTEGER;
    trad_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO news_count FROM noticias;
    SELECT COUNT(*) INTO trad_count FROM traducciones;
    RAISE NOTICE 'Base de datos limpiada: % noticias, % traducciones', news_count, trad_count;
END $$;
