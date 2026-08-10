BEGIN;

-- Columnas para el clustering de noticias (cluster_worker.py)
ALTER TABLE eventos
    ADD COLUMN IF NOT EXISTS centroid JSONB,
    ADD COLUMN IF NOT EXISTS total_traducciones INTEGER DEFAULT 1,
    ADD COLUMN IF NOT EXISTS fecha_inicio TIMESTAMP,
    ADD COLUMN IF NOT EXISTS fecha_fin TIMESTAMP,
    ADD COLUMN IF NOT EXISTS n_noticias INTEGER DEFAULT 1;

COMMIT;
