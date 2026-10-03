-- 46-entity-blocklist.sql
-- Lista de falsos positivos del NER (entidades y temas) que la API no debe
-- mostrar y que `ner_worker` no debe volver a insertar.
--
-- La llena `backend/cmd/entityscan` aplicando las reglas de
-- `backend/internal/entitycheck` sobre TODOS los valores de `tags` (todos los
-- países y tipos), y la leen:
--   * handlers.GetEntities / handlers.GetEntityNews  → Populares deja de
--     mostrar el valor (también cambia `total`, así que la paginación cuadra);
--   * workers/ner_worker.py                          → no lo inserta de nuevo.
--
-- Idempotente. El binario /entityscan también la crea si falta, así que esta
-- migración es opcional pero recomendable antes del primer escaneo.
--
--   make entities-scan          # informe (dry-run)
--   make entities-scan-apply    # siembra la tabla
--   /entityscan -unblock "X" -tipo persona   # deshacer un valor concreto

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
