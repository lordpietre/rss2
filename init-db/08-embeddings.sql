CREATE TABLE IF NOT EXISTS traduccion_embeddings (
  id             SERIAL PRIMARY KEY,
  traduccion_id  INT NOT NULL REFERENCES traducciones(id) ON DELETE CASCADE,
  model          TEXT NOT NULL,
  dim            INT NOT NULL,
  embedding      DOUBLE PRECISION[] NOT NULL,
  created_at     TIMESTAMP DEFAULT NOW(),
  UNIQUE (traduccion_id, model)
);

CREATE INDEX IF NOT EXISTS idx_tr_emb_traduccion_id ON traduccion_embeddings(traduccion_id);
CREATE INDEX IF NOT EXISTS idx_tr_emb_model          ON traduccion_embeddings(model);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'embeddings'
  ) THEN
    EXECUTE 'ALTER TABLE embeddings RENAME TO embeddings_legacy';
  END IF;
EXCEPTION WHEN others THEN
  NULL;
END$$;

CREATE OR REPLACE VIEW embeddings AS
SELECT
  te.traduccion_id,
  te.dim,
  te.embedding AS vec
FROM traduccion_embeddings te
WHERE te.model = 'sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2';

CREATE TABLE IF NOT EXISTS related_noticias (
  id                      SERIAL PRIMARY KEY,
  noticia_id              VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
  related_id              VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
  traduccion_id           INT REFERENCES traducciones(id) ON DELETE CASCADE,
  score                   DOUBLE PRECISION NOT NULL,
  created_at              TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_related_by_tr        ON related_noticias (traduccion_id);
CREATE INDEX IF NOT EXISTS idx_related_by_relatedid ON related_noticias (related_id);

