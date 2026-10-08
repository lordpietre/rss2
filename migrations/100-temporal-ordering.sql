-- Migration: Temporal Ordering
-- Adds created_at to noticias, last_used to saved_searches, temporal fields to tags

BEGIN;

-- 1. Add created_at to noticias (for scrape time tracking)
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();
CREATE INDEX IF NOT EXISTS idx_noticias_created_at ON noticias(created_at DESC);

-- 2. Add last_used and use_count to saved_searches for temporal sorting
ALTER TABLE user_saved_searches ADD COLUMN IF NOT EXISTS last_used TIMESTAMP;
ALTER TABLE user_saved_searches ADD COLUMN IF NOT EXISTS use_count INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_saved_searches_last_used ON user_saved_searches(last_used DESC);

-- 3. Add temporal fields to tags for entity sorting
ALTER TABLE tags ADD COLUMN IF NOT EXISTS recent_cnt_7d INTEGER DEFAULT 0;
ALTER TABLE tags ADD COLUMN IF NOT EXISTS recent_cnt_30d INTEGER DEFAULT 0;
ALTER TABLE tags ADD COLUMN IF NOT EXISTS last_mentioned_at TIMESTAMP;
CREATE INDEX IF NOT EXISTS idx_tags_last_mentioned ON tags(last_mentioned_at DESC);
CREATE INDEX IF NOT EXISTS idx_tags_recent_cnt_30d ON tags(recent_cnt_30d DESC);

COMMIT;
