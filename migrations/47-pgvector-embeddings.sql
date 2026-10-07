-- Migration: Add pgvector extension and migrate embeddings to vector type
-- This enables native vector search in PostgreSQL

-- 1. Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector CASCADE;

-- 2. Add the vector column with proper dimension for the model
-- Model: paraphrase-multilingual-MiniLM-L12-v2 outputs 384 dimensions
ALTER TABLE traduccion_embeddings 
ADD COLUMN IF NOT EXISTS embedding_vector vector(384);

-- 3. Migrate existing data from array to vector type
-- The existing embedding column (DOUBLE PRECISION[]) is preserved for backwards compatibility
-- but the new embedding_vector column will be used for searches
UPDATE traduccion_embeddings 
SET embedding_vector = embedding::vector(384)
WHERE embedding_vector IS NULL AND embedding IS NOT NULL;

-- 4. Create HNSW index for fast approximate nearest neighbor search
-- HNSW is optimal for cosine distance searches
-- Using cosine distance (<=>) which is what pgvector uses for normalized vectors
CREATE INDEX IF NOT EXISTS idx_tr_emb_vector_hnsw 
ON traduccion_embeddings 
USING hnsw (embedding_vector vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- 5. Create index on the original array column as fallback
CREATE INDEX IF NOT EXISTS idx_tr_emb_embedding_gin 
ON traduccion_embeddings USING gin (embedding);

-- 6. Function to search for similar translations using pgvector
-- This replaces the in-memory cosine similarity calculation
CREATE OR REPLACE FUNCTION find_related_translations(
    query_embedding vector(384),
    match_count int DEFAULT 10,
    match_threshold float DEFAULT 0.0,
    exclude_id int DEFAULT NULL
) RETURNS TABLE (
    traduccion_id int,
    score float
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        te.traduccion_id,
        1 - (te.embedding_vector <=> query_embedding) AS score
    FROM traduccion_embeddings te
    WHERE te.embedding_vector IS NOT NULL
      AND (exclude_id IS NULL OR te.traduccion_id != exclude_id)
    ORDER BY te.embedding_vector <=> query_embedding
    LIMIT match_count;
END;
$$ LANGUAGE plpgsql STABLE;

-- 7. Grant permissions
GRANT USAGE ON EXTENSION vector TO PUBLIC;
GRANT EXECUTE ON FUNCTION find_related_translations TO PUBLIC;

-- 8. Comments for documentation
COMMENT ON EXTENSION vector IS 'PostgreSQL vector similarity search - required for embeddings search';
COMMENT ON COLUMN traduccion_embeddings.embedding_vector IS '384-dimensional embedding vector for semantic similarity search using pgvector';
COMMENT ON FUNCTION find_related_translations IS 'Find related translations using cosine similarity with pgvector - replaces in-memory calculation';
