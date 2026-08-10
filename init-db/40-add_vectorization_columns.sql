ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS vectorization_date TIMESTAMP;
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS qdrant_point_id TEXT;
