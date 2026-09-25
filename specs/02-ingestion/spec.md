# Spec 02 — Ingesta (feeds → noticias)

## Alcance

`rss-ingestor-go` (servicio `ingestor`): lee `feeds` activos, descarga RSS
(`gofeed`), inserta en `noticias` con `id = md5(url)`, `ON CONFLICT DO NOTHING`.
`getActiveURLs` sin `LIMIT`; concurrencia `RSS_MAX_WORKERS=20` (compose),
revisita cada `RSS_POKE_INTERVAL_MIN=8`, timeout `RSS_FEED_TIMEOUT=60`.

## Env (compose, obligatorias)

`DB_HOST=db DB_PORT=5432 DB_NAME/DB_USER=${POSTGRES_*:-rss}
DB_PASS=${POSTGRES_PASSWORD}` (+ `RSS_MAX_WORKERS`, `RSS_POKE_INTERVAL_MIN`).

## Criterios de aceptación

- [x] `SELECT count(*) FROM noticias` crece de forma sostenida.
- [x] Feeds muertos (522/timeout/DNS) no tumban el worker (log + sigue).
- [x] Re-ejecuciones no duplican (`UNIQUE(url)` + md5 idempotente).
- [ ] Métrica de cobertura: % feeds activos con ≥1 noticia de 7 días
      (hoy sin exponer; propuesta en tasks).

## Plan

1. Mantener servicio `ingestor` con `depends_on db healthy` (hecho).
2. Añadir endpoint o log periódico de cobertura por feed (pendiente).
3. Si se quieren semillas nuevas: importar CSV por `/api/feeds/import`
   (requiere multipart con boundary; ver spec 06) o sembrar `fuentes_url`
   para `discovery` (spec 07).

## Tasks

- [x] Servicio `ingestor` en compose con credenciales reales.
- [x] Verificar crecimiento de `noticias` y ausencia de duplicados.
- [x] `GET /api/admin/ingest/stats` implementado (2026-09-14): feeds
      total/activos/con fallos, noticias total/7d, cobertura % 7d.
- [x] Poda 2026-09-14: eliminados 5320 feeds (`activo=false` + `fallos≥30`,
      respaldo CSV en `/tmp/opencode/feeds_muertos_backup.csv`; sin FKs
      hacia `feeds`, borrado seguro). Quedan 1502 (1161 activos).
- [x] Endurecido `ON CONFLICT` 2026-09-14: ingestor `(url)` y scraper
      `(id)` tumbaban el chunk ante duplicados cruzados
      (`duplicate key noticias_pkey` visto en logs). Ahora
      `ON CONFLICT DO NOTHING` sin árbitro en ambos: cualquier duplicado
      salta la fila sin perder el lote.
