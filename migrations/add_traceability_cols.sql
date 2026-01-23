-- Add fuente_url_id to feeds table for traceability
ALTER TABLE feeds 
ADD COLUMN IF NOT EXISTS fuente_url_id INTEGER REFERENCES fuentes_url(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_feeds_fuente_url ON feeds(fuente_url_id);

COMMENT ON COLUMN feeds.fuente_url_id IS 'ID of the URL source that discovered this feed';
