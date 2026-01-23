-- Migration: Add pending feeds table for review workflow
-- This table stores discovered feeds that need manual review/approval

CREATE TABLE IF NOT EXISTS feeds_pending (
    id SERIAL PRIMARY KEY,
    fuente_url_id INTEGER REFERENCES fuentes_url(id) ON DELETE CASCADE,
    feed_url TEXT NOT NULL UNIQUE,
    feed_title VARCHAR(255),
    feed_description TEXT,
    feed_language CHAR(5),
    feed_type VARCHAR(20),
    entry_count INTEGER DEFAULT 0,
    detected_country_id INTEGER REFERENCES paises(id),
    suggested_categoria_id INTEGER REFERENCES categorias(id),
    categoria_id INTEGER REFERENCES categorias(id),
    pais_id INTEGER REFERENCES paises(id),
    idioma CHAR(2),
    discovered_at TIMESTAMP DEFAULT NOW(),
    reviewed BOOLEAN DEFAULT FALSE,
    approved BOOLEAN DEFAULT FALSE,
    reviewed_at TIMESTAMP,
    reviewed_by VARCHAR(100),
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_feeds_pending_reviewed ON feeds_pending(reviewed, approved);
CREATE INDEX IF NOT EXISTS idx_feeds_pending_fuente ON feeds_pending(fuente_url_id);

-- Add constraint to fuentes_url to require categoria_id or pais_id for processing
ALTER TABLE fuentes_url 
    ADD COLUMN IF NOT EXISTS require_review BOOLEAN DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS auto_approve BOOLEAN DEFAULT FALSE;

COMMENT ON TABLE feeds_pending IS 'Feeds discovered but pending review/approval before being added to active feeds';
COMMENT ON COLUMN feeds_pending.detected_country_id IS 'Country detected automatically from feed language/domain';
COMMENT ON COLUMN feeds_pending.suggested_categoria_id IS 'Category suggested based on feed content/keywords';
COMMENT ON COLUMN fuentes_url.require_review IS 'If TRUE, feeds from this URL need manual approval';
COMMENT ON COLUMN fuentes_url.auto_approve IS 'If TRUE, feeds are automatically approved and activated';
