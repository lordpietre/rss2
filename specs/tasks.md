# Tasks consolidadas

Estado global. El detalle y la evidencia están en cada `specs/NN-*/spec.md`.

## Hecho [x]

- [x] Fix `logger.Info/Error/...` + `func main` en workers Go (01, 05).
- [x] Go 1.23.12 instalado en `/usr/local/go` + PATH.
- [x] Ollama fuera del compose (a petición) + `OLLAMA_URL` fuera.
- [x] Imports `log` sin usar + `logger.Printf` + línea suelta en discovery.
- [x] Credenciales DB/Redis cableadas a translator(+gpu)/scheduler/langdetect/ner/ingestor.
- [x] Trusted proxies gin (rate limit por IP real).
- [x] Interceptor multipart en axios (import CSV + restore + aliases).
- [x] Fix `n.contenido`, args SQL duplicados, scans vs esquema real, `ID string`.
- [x] Fix orden de columnas en `GetEntities`.
- [x] Fix doble-encode de caché (`cache.Set` con structs) + `FLUSHDB`.
- [x] Register duplicado → 409 + mensaje ES en login.
- [x] Servicios `ingestor`, `translation-scheduler`, `langdetect`, `ner`.
- [x] `Dockerfile.scheduler` con `workers/` completo; `Dockerfile.ner` nuevo.
- [x] `.dockerignore` raíz (data/models/hf_cache/backups).
- [x] `translator-gpu` a profile `gpu` (sin NVIDIA aquí).
- [x] Upstreams nginx a `backend`.
- [x] Rotación de credenciales: procedimiento `ALTER USER` + recreate.
- [x] Reset de password admin + verificación JWT.
- [x] Fixes TS frontend (AdminSettings duplicado/export real, Populares Set).

## Pendiente (prioridad)

- [x] Gap AdminAliases → implementados `GET /admin/aliases`, `PUT/DELETE
      /admin/aliases/:id` + `GET /admin/ingest/stats` (2026-09-14, verificado
      en vivo: 401 sin auth = rutas registradas).
- [x] `ErrorBoundary` frontend (`components/ErrorBoundary.tsx`, envuelve
      `Routes` en `App.tsx`; build Docker OK).
- [x] Healthchecks workers (ingestor/scheduler/langdetect/ner/translator/
      wiki/topics vía `grep -q … /proc/1/cmdline`; todos `healthy`).
- [x] Fix `workers.Connect`: ningún worker Go llamaba a `db.Connect`
      (pool nil → crash-loop al activar). Ahora construye el DSN desde
      `DB_*` con password URL-escaped. Afecta a topics/wiki/scraper/
      discovery/related/qdrant.
- [x] Fix scanner alertas: comparaba el día en curso (incompleto) y nunca
      disparaba. Ahora usa el último día completo (`fecha < CURRENT_DATE`).
      Verificado: 1044 alertas del periodo 2026-09-13.
- [x] Activados `wiki` y `topics` (reutilizan imagen `rss2-backend`, sin
      build extra; `Dockerfile.wiki` queda legacy por contexto raíz pesado).
      Wiki enriquece (`Putin` ya tiene `wiki_summary`); topics procesa
      500/batch (`news_topics` creciendo).
- [x] Eliminado `backend/cmd/main.go` roto (importaba paquete main; rompía
      `go vet ./...`). `Dockerfile` compila cada `cmd/*` por separado.
- [x] Activados `related`, `qdrant-worker`, `scraper`, `discovery`
      (2026-09-14, reutilizan `rss2-backend` + `entrypoint`, todos
      `healthy`): related 6000+ filas, qdrant subiendo a `news_vectors`,
      scraper enriqueciendo, discovery en espera (`fuentes_url` vacía).
- [x] Embeddings local (servicio `embeddings`, `Dockerfile.embeddings`,
      MiniLM-L12-v2 desde `hf_cache`; `EMB_MODEL` igual en related).
- [x] Métricas 429 (`GET /api/stats/ratelimit`, contadores live por
      configuración; verificado: total 0).
- [x] `WORKER_*_ENABLED` + `TRANSLATOR_URL/_GPU` + `QDRANT_URL` muertos
      eliminados del compose (nada los consumía; `config.go` como prueba).
- [x] Fix healthcheck `wiki` (`wiki-worker` con guion, no `wiki_worker`).
- [x] Limpieza disco raíz 100%→85% (cachés + contenedores parados +
      imagen gpu; ver spec 08).
- [x] Qdrant eliminado por completo (2026-09-15): verificado que nada lo
      leía (`SemanticSearch` stub vacío, sin UI semántica, related por
      SQL). Fuera: servicios `qdrant`+`qdrant-worker`, `backend/cmd/qdrant`,
      build en `backend/Dockerfile`, `QdrantHost/Port` en config,
      `qdrant-client` en requirements, targets Makefile, checks en scripts,
      vars `QDRANT_*` (.env + examples + generator), ignores y 500M de
      `data/qdrant_storage`. Migraciones `40-add_vectorization_columns.sql`
      se conservan (históricas; columnas inofensivas).
- [x] Código muerto Python eliminado (2026-09-15, 2200 líneas): `cluster`,
      `llm_categorizer`, `remote_translator`, `simple_categorizer`,
      `simple_translator[.py]`, `translation_worker` (0 referencias vivas;
      respaldo en `/tmp/opencode/dead_workers_backup_20260915.tgz` +
      reversible vía git). Quedan los 5 workers en uso.
- [ ] Backup pre-rotación (sigue manual).
