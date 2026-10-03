-- migrations/add_entity_blocklist.sql
-- Equivalente de init-db/46-entity-blocklist.sql para bases ya existentes.
-- Aplicar con:
--   docker compose exec -T db psql -U rss -d rss < migrations/add_entity_blocklist.sql
-- (el binario /entityscan también la crea si falta).

CREATE TABLE IF NOT EXISTS entity_blocklist (
    id          BIGSERIAL PRIMARY KEY,
    tipo        TEXT      NOT NULL,
    valor       TEXT      NOT NULL,
    valor_lower TEXT      NOT NULL,
    motivo      TEXT      NOT NULL,
    menciones   INT       NOT NULL DEFAULT 0,
    origen      TEXT      NOT NULL DEFAULT 'entityscan',
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (tipo, valor_lower)
);

CREATE INDEX IF NOT EXISTS idx_entity_blocklist_tipo ON entity_blocklist (tipo);

COMMENT ON TABLE entity_blocklist IS
    'FP del NER ya detectados (entitycheck): la API no los muestra y ner_worker no los inserta';
