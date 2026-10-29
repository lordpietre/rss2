-- =============================================================================
-- RSS2 - Limpieza de Noticias y Traducciones
-- Para empezar de cero con el proceso de traducción
-- =============================================================================
--
-- IMPORTANTE: este script se ejecuta durante la inicializacion de la base
-- (el entrypoint de Postgres corre los ficheros de init-db/ en orden con
-- ON_ERROR_STOP=1). Si algo falla aqui, la inicializacion ABORTA y las
-- migraciones siguientes (11...64) no se aplican nunca.
--
-- Por eso todo es condicional: en una base recien creada algunas tablas y
-- secuencias todavia no existen. Antes fallaba con
--   ERROR: relation "noticias_id_seq" does not exist
-- (noticias.id es VARCHAR(32), no un SERIAL: no tiene secuencia) y la base
-- quedaba a medias.

-- Desactivar triggers para speed
SET session_replication_role = 'replica';

-- Truncar solo las tablas que ya existen. El orden va de mayor a menor
-- dependencia por FK.
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'related_noticias', 'tags_noticia', 'eventos_noticias', 'news_topics',
        'traduccion_embeddings', 'noticias', 'traducciones', 'translation_stats'
    ] LOOP
        IF to_regclass(t) IS NOT NULL THEN
            EXECUTE format('TRUNCATE TABLE %I RESTART IDENTITY CASCADE', t);
        END IF;
    END LOOP;
END $$;

-- Reactivar triggers
SET session_replication_role = 'origin';

-- Reset de las secuencias que existan (no todas las tablas tienen SERIAL:
-- noticias.id es VARCHAR(32) y no genera secuencia).
DO $$
DECLARE
    seq text;
BEGIN
    FOREACH seq IN ARRAY ARRAY[
        'noticias_id_seq', 'traducciones_id_seq', 'related_noticias_id_seq',
        'tags_noticia_id_seq', 'eventos_noticias_id_seq', 'news_topics_id_seq',
        'traduccion_embeddings_id_seq', 'translation_stats_id_seq'
    ] LOOP
        IF to_regclass(seq) IS NOT NULL THEN
            EXECUTE format('SELECT setval(%L, 1, false)', seq);
        END IF;
    END LOOP;
END $$;
