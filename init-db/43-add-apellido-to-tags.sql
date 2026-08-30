-- Add apellido column to tags table for linking personalities by last name
ALTER TABLE tags ADD COLUMN IF NOT EXISTS apellido VARCHAR(100);

-- Index for efficient last name lookups
CREATE INDEX IF NOT EXISTS idx_tags_apellido ON tags(apellido) WHERE apellido IS NOT NULL;