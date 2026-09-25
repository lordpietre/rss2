# Plan global — consolidación y backlog

## Fase 0 — Estabilidad (completada)

Sistema levantado end-to-end: ingesta → lang → pending → done → tags →
API → frontend, con credenciales rotadas y verificadas. Todo lo de esta
fase está marcado [x] en los specs 01–08.

## Fase 1 — Deuda conocida (completada 2026-09-14)

1. [x] Gap AdminAliases: implementados `GET/PUT/DELETE /admin/aliases/:id`.
2. [x] `ErrorBoundary` en frontend.
3. [x] Healthchecks en todos los workers.
4. [x] Backup pre-rotación: retención 30 días + `pipefail` + descarta fallidos.
5. [x] `GET /api/admin/ingest/stats` + poda de 5320 feeds muertos.
6. [x] `alertas` funcionando (fix día completo; 1000+ por periodo).

## Fase 2 — Activaciones (completada 2026-09-14/15)

1. [x] `wiki` → tooltips con imagen.
2. [x] `topics` → `pais_id` y scores.
3. [x] Embeddings local (MiniLM) → `related` por coseno SQL.
   Qdrant eliminado 2026-09-15 (nadie lo leía; ver tasks).
4. [x] `scraper` + `discovery` activos (`fuentes_url` vacía: scraper
   enriquece, discovery en espera; sembrar URLs es operativo).

## Fase 3 — Endurecimiento (completada 2026-09-14)

1. [x] `GET /api/stats/ratelimit` (429s totales y por configuración).
2. [x] `WORKER_*_ENABLED` y URLs muertas eliminados del compose.
3. [x] Protocolo remote-worker referenciado en spec 01.
4. [x] `cmd/main.go` roto eliminado.

## Restante (operativo, no código)

- Sembrar `fuentes_url` si se quiere discovery con trabajo.
- Vigilar disco (`backups/` crece ~0.9G/día) y backlog de traducción CPU.
