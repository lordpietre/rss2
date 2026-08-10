BEGIN;

-- El worker related (cmd/related/main.go) trabaja con traducciones
ALTER TABLE related_noticias
    ADD COLUMN IF NOT EXISTS related_traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_related_tr_pair
    ON related_noticias(traduccion_id, related_traduccion_id);

COMMIT;
