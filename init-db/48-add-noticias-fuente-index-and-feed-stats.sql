-- Optimización de consultas de feeds
-- 1. Índice en fuente_nombre para acelerar el COUNT(*) en GetFeeds
CREATE INDEX IF NOT EXISTS idx_noticias_fuente_nombre ON noticias(fuente_nombre);

-- 2. Tabla materializada para estadísticas de feeds (noticias_count por feed)
-- Esta tabla se actualiza mediante triggers en noticias
CREATE TABLE IF NOT EXISTS feed_stats (
    fuente_nombre VARCHAR(255) PRIMARY KEY,
    noticias_count BIGINT NOT NULL DEFAULT 0
);

-- Trigger para mantener feed_stats actualizada
CREATE OR REPLACE FUNCTION update_feed_stats() RETURNS TRIGGER AS $$
DECLARE
    v_count BIGINT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO feed_stats (fuente_nombre, noticias_count) 
        VALUES (NEW.fuente_nombre, 1)
        ON CONFLICT (fuente_nombre) DO UPDATE
        SET noticias_count = feed_stats.noticias_count + 1;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE feed_stats SET noticias_count = noticias_count - 1
        WHERE fuente_nombre = OLD.fuente_nombre;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_feed_stats ON noticias;
CREATE TRIGGER trigger_update_feed_stats
    AFTER INSERT OR DELETE ON noticias
    FOR EACH ROW EXECUTE FUNCTION update_feed_stats();

-- Inicializar feed_stats con los datos existentes
INSERT INTO feed_stats (fuente_nombre, noticias_count)
SELECT fuente_nombre, COUNT(*) 
FROM noticias 
WHERE fuente_nombre IS NOT NULL AND fuente_nombre != ''
GROUP BY fuente_nombre
ON CONFLICT (fuente_nombre) DO UPDATE 
SET noticias_count = EXCLUDED.noticias_count;
