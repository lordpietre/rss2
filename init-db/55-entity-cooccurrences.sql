-- Entity Co-occurrences Table
-- Pre-computed entity co-occurrences for graph visualization
-- This table is updated periodically by a background job, not in real-time

CREATE TABLE IF NOT EXISTS entity_cooccurrences (
    id SERIAL PRIMARY KEY,
    tag1_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    tag2_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    co_count INTEGER NOT NULL DEFAULT 0,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tag1_id, tag2_id)
);

-- Index for fast lookups by tag
CREATE INDEX IF NOT EXISTS idx_coocc_tag1 ON entity_cooccurrences(tag1_id);
CREATE INDEX IF NOT EXISTS idx_coocc_tag2 ON entity_cooccurrences(tag2_id);
CREATE INDEX IF NOT EXISTS idx_coocc_count ON entity_cooccurrences(co_count DESC);

-- Function to refresh co-occurrences for a specific tag
CREATE OR REPLACE FUNCTION refresh_entity_cooccurrences_for_tag(p_tag_id INTEGER)
RETURNS VOID AS $$
BEGIN
    -- Remove existing co-occurrences for this tag
    DELETE FROM entity_cooccurrences WHERE tag1_id = p_tag_id OR tag2_id = p_tag_id;
    
    -- Insert new co-occurrences
    INSERT INTO entity_cooccurrences (tag1_id, tag2_id, co_count, last_updated)
    SELECT 
        LEAST(tn1.tag_id, tn2.tag_id) AS tag1_id,
        GREATEST(tn1.tag_id, tn2.tag_id) AS tag2_id,
        COUNT(DISTINCT tn1.noticia_id) AS co_count
    FROM tags_noticia tn1
    JOIN tags_noticia tn2 ON tn1.noticia_id = tn2.noticia_id AND tn1.tag_id < tn2.tag_id
    WHERE tn1.tag_id = p_tag_id OR tn2.tag_id = p_tag_id
    GROUP BY LEAST(tn1.tag_id, tn2.tag_id), GREATEST(tn1.tag_id, tn2.tag_id)
    HAVING COUNT(DISTINCT tn1.noticia_id) >= 2
    ON CONFLICT (tag1_id, tag2_id) DO UPDATE SET
        co_count = EXCLUDED.co_count,
        last_updated = NOW();
END;
$$ LANGUAGE plpgsql;

-- Function to refresh all co-occurrences (full rebuild)
-- WARNING: This can be slow for large datasets
CREATE OR REPLACE FUNCTION refresh_all_entity_cooccurrences()
RETURNS VOID AS $$
BEGIN
    -- Clear existing data
    TRUNCATE entity_cooccurrences;
    
    -- Insert all co-occurrences from recent news (last 30 days)
    -- This is much faster than scanning all history
    INSERT INTO entity_cooccurrences (tag1_id, tag2_id, co_count, last_updated)
    SELECT 
        LEAST(tn1.tag_id, tn2.tag_id) AS tag1_id,
        GREATEST(tn1.tag_id, tn2.tag_id) AS tag2_id,
        COUNT(DISTINCT tn1.noticia_id) AS co_count
    FROM tags_noticia tn1
    JOIN tags_noticia tn2 ON tn1.noticia_id = tn2.noticia_id AND tn1.tag_id < tn2.tag_id
    JOIN noticias n ON tn1.noticia_id = n.id
    WHERE n.fecha >= NOW() - INTERVAL '30 days'
    GROUP BY LEAST(tn1.tag_id, tn2.tag_id), GREATEST(tn1.tag_id, tn2.tag_id)
    HAVING COUNT(DISTINCT tn1.noticia_id) >= 3
    ON CONFLICT (tag1_id, tag2_id) DO UPDATE SET
        co_count = EXCLUDED.co_count,
        last_updated = NOW();
END;
$$ LANGUAGE plpgsql;

-- Initial population (optional, can be run manually)
-- SELECT refresh_all_entity_cooccurrences();

COMMENT ON TABLE entity_cooccurrences IS 'Pre-computed entity co-occurrences for graph visualization. Updated periodically by background job.';
