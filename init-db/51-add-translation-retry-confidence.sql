-- Add retry_count and confidence columns to traducciones for improved error handling

ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS retry_count INTEGER DEFAULT 0;
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION;

-- Index for finding pending translations with retry_count to enable efficient retry queries
CREATE INDEX IF NOT EXISTS idx_traducciones_status_retry ON traducciones(status, retry_count) WHERE status = 'pending';
