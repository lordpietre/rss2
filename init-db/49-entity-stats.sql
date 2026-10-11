-- Migration: Create entity_stats table for fast entity popular queries
-- This table stores pre-computed entity counts and wiki info

CREATE TABLE IF NOT EXISTS entity_stats (
    id SERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL,
    canonical_name TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    last_date TIMESTAMP,
    wiki_summary TEXT,
    wiki_url TEXT,
    image_path TEXT,
    apellido TEXT,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(entity_type, canonical_name)
);

CREATE INDEX IF NOT EXISTS idx_entity_stats_type_count ON entity_stats(entity_type, count DESC);
CREATE INDEX IF NOT EXISTS idx_entity_stats_name ON entity_stats(entity_type, canonical_name);

-- Function to refresh entity_stats
CREATE OR REPLACE FUNCTION refresh_entity_stats() RETURNS void AS $$
BEGIN
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
    
    UPDATE entity_stats SET updated_at = NOW();
END;
$$ LANGUAGE plpgsql;
