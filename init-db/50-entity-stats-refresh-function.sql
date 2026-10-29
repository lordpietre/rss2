-- Migration: Add entity_stats refresh cron function
-- This allows scheduling entity_stats refresh via pg_cron

-- pg_cron no viene en la imagen pgvector/pgvector:pg16 (esta disponible en
-- el paquete de PostgreSQL, no como extension). Con 'CREATE EXTENSION' a
-- pelo la migracion ABORTABA en esta linea (el init de Docker corre psql
-- con ON_ERROR_STOP=1) y la funcion de abajo nunca se creaba: el fichero
-- quedaba a medias.
--
-- La funcion se crea igualmente: la puede llamar el backend (que ya
-- refresca entity_stats al arrancar) o cualquier planificador externo.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_cron') THEN
        CREATE EXTENSION IF NOT EXISTS pg_cron;
    ELSE
        RAISE NOTICE 'pg_cron no disponible en esta imagen: se omite la extension (la funcion se crea igual)';
    END IF;
END $$;

-- Function to refresh entity_stats (idempotent, can be called hourly)
CREATE OR REPLACE FUNCTION cron_refresh_entity_stats() RETURNS void AS $$
BEGIN
    -- Use TRUNCATE + INSERT for atomic refresh
    TRUNCATE entity_stats;
    
    INSERT INTO entity_stats (entity_type, canonical_name, count, last_date, wiki_summary, wiki_url, image_path, apellido)
    SELECT 
        t.tipo,
        COALESCE(ea.canonical_name, t.valor),
        COUNT(*)::int,
        MAX(n.fecha),
        MAX(t.wiki_summary),
        MAX(t.wiki_url),
        MAX(t.image_path),
        MAX(t.apellido)
    FROM tags_noticia tn
    JOIN tags t ON tn.tag_id = t.id
    JOIN traducciones tr ON tn.traduccion_id = tr.id
    JOIN noticias n ON tr.noticia_id = n.id
    LEFT JOIN entity_aliases ea ON LOWER(ea.alias) = LOWER(t.valor) AND ea.tipo = t.tipo
    WHERE NOT EXISTS (
        SELECT 1 FROM entity_blocklist b 
        WHERE b.tipo = t.tipo AND b.valor_lower = LOWER(t.valor)
    )
    GROUP BY t.tipo, COALESCE(ea.canonical_name, t.valor);
    
    -- Update timestamp
    UPDATE entity_stats SET updated_at = NOW();
END;
$$ LANGUAGE plpgsql;

-- Schedule refresh every hour (if pg_cron is available)
-- Note: pg_cron runs in the database, not in the application
-- SELECT cron.schedule('entity-stats-refresh', '0 * * * *', 'SELECT cron_refresh_entity_stats()');
