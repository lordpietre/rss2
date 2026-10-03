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
- [x] Limpieza de textos (2026-09-30): paquete `textclean` (parser
      `x/net/html` + RE2, idempotente y acotado) en ingestor y scraper,
      backfill `backend/cmd/sanitize` (`dry-run` / `-apply` / `-requeue` /
      `-restore`), columnas `noticias.titulo_raw|resumen_raw|cleaned_at` y
      traducción con `lang_from=NULL` → se detecta sobre el texto limpio.
      Detalle y métricas en `specs/02-ingestion/spec.md`.

- [x] Falsos positivos de entidades (2026-09-30): reglas únicas en
      `backend/internal/entitycheck`, blocklist global
      (`entity_blocklist` + `cmd/entityscan`), filtro en `/api/entities`
      (lista y total) y en `/entities/news`, `ner_worker` que no los
      reinserta y tests de API dentro de `make test` (lo más popular de
      España + global). 7.248 valores (2,4 %) fuera. Plan y fases
      pendientes (alias/fragmentación, FP débiles, wiki) en
      `specs/04-enrichment/spec.md`.

## Pendiente (prioridad)

- [x] **Documentación de arquitectura** (2026-10-01): Creados 6 documentos en
      `specs/00-architecture/`:
      - 01-system-context.md: Contexto, usuarios, sistemas externos
      - 02-container-view.md: Topología Docker completa
      - 03-data-flow.md: Pipeline de datos y limpieza de texto
      - 04-component-inventory.md: Componentes, APIs, esquemas BD
      - 05-spec-driven-development.md: Metodología SDD
      - 06-roadmap.md: Estado actual y roadmap Q4
      Ver `specs/00-architecture/README.md` para índice.

- [ ] **Alertas: racionalizar el algoritmo** (plan completo en
      `specs/04-enrichment/spec.md`, sección «Alertas: revisión del
      algoritmo (2026-09-30)»). Medido: 843 alertas en 5 días (169/día),
      todas sin leer, 74 % con `baseline<1`, 19 % no significativas, 65 %
      temas genéricos, 4/843 en la blocklist y `/api/alerts` sin filtrar.
      Valores propuestos (backtest 118 h): `hits≥5`, `baseline≥1`,
      `ratio≥5`, `buckets≥2`, `tema 6/2/6/nb3`, cooldown 6 h, filtro
      `entity_blocklist`+`entitycheck`, scan 60 min → **21,5 alertas/día
      y 0 % no significativas** (fases A1-A6 en la spec; A1a es solo env:
      `ALERTS_MIN_HITS=5 ALERTS_MIN_BASELINE=1`).

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
- [x] Fix alertas por HORA (2026-09-26): la petición fue pasar de día a
      hora. Con la base reconstruida solo había 1 día completo, así que
      `AVG(COALESCE(cnt,0))` sobre la serie de 8 días daba `baseline=0`
      en las 67 430 entidades y `baseline≥2` las descartaba todas → 0
      alertas en silencio. Ahora detecta picos **por hora** (9 buckets
      activos vs 1), el baseline solo promedia buckets con datos (no
      cuenta huecos como 0 menciones) y se ajusta al volumen de la hora
      ref, donde sin ajustar el ratio máximo medido era 1.8 y nada
      llegaba a umbral. `alertas.periodo` DATE→TIMESTAMP (migración
      idempotente en el arranque). Verificado: 29 alertas, `ratio`
      coherente con `hits/baseline`.
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
