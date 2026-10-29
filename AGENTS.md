# RSS2 — Agent Instructions

## Architecture

- **Go backend** (`backend/`) + **Go workers** (scraper, discovery, topics, wiki, related, sanitize, entityscan)
- **Go ingestor** (`rss-ingestor-go/`) — separate module for RSS feed ingestion
- **Python workers** (`workers/`, `translator/`) — ner, langdetect, translation-scheduler, ctranslator, embeddings
- **React frontend** (`frontend/`) — Vite + TypeScript + Tailwind
- **PostgreSQL 16 + pgvector** — primary store; **Redis** — cache
- **nginx** — single public port `:8888` (gateway for backend + frontend)
- **Optional**: `monitoring` profile (prometheus, grafana, cadvisor); `gpu` profile (translator-gpu)

## Developer Commands

### Go (backend + rss-ingestor-go)
```bash
make all         # deps + build (full local build)
make deps        # go mod download + tidy (backend/ only; rss-ingestor-go/ is untouched)
make build       # build all Go binaries to bin/
make clean       # remove bin/ + go clean
make test        # go test ./... in both modules
make parity      # diff backend/internal/textclean rss-ingestor-go/textclean (must stay identical)
```

### Go workers (run locally with DB env — binaries in bin/, no Docker)
```bash
make run-scraper     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-discovery   DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-topics      DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-related     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss RELATED_SLEEP=10
# wiki: no make target; run manually:
#   DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss \
#   REDIS_HOST=localhost REDIS_PORT=6379 REDIS_PASSWORD=<pass> WIKI_SLEEP=5 WIKI_BATCH=40 \
#   bin/wiki-worker
```

### Sanitize + entity operations (affects production data)
```bash
make sanitize-dry-run         # preview text cleaning
make sanitize                # apply + requeue for translation
make sanitize-restore        # undo from noticias.*_raw
make sanitize-local          # run locally (requires Go + direct DB access)

make entities-scan           # dry-run NER false-positive scan
make entities-scan-apply     # seed entity_blocklist from scan
make entities-scan-local     # run locally (Go + direct DB)

make alerts-prune            # dry-run legacy alert pruning
make alerts-prune-apply     # apply alert pruning
make alerts-prune-local      # run locally (Go + direct DB)
```

### Frontend
```bash
cd frontend
npm test                     # vitest
npm run build                # tsc && vite build
```

### Docker startup
```bash
cp .env.example .env         # fill in POSTGRES_PASSWORD, REDIS_PASSWORD, SECRET_KEY
docker compose up -d --build backend   # first: build shared image
docker compose up -d --no-build        # then: rest of services
```

### Docker builds (two-step, avoids AlreadyExists conflicts)
```bash
docker compose build backend frontend ingestor translator
docker compose build translation-scheduler langdetect ner embeddings
```

## Critical Quirks

- **textclean duplication**: `backend/internal/textclean/` and `rss-ingestor-go/textclean/` must be identical. Use `make parity` to verify after changes. They build in separate Docker contexts and would diverge without this check. **Always run `make parity` before committing textclean changes** — there is no CI/pre-commit hook enforcing this.

- **Go workers reuse backend image**: Workers (scraper, discovery, topics, wiki, related) are built into the `backend` Dockerfile as separate binaries (`/scraper`, `/discovery`, etc.) and then invoked with different entrypoints in `docker-compose.yml`. Never `docker compose build` two services against the same image tag in one command (conflicts with `AlreadyExists`).

- **Postgres shm_size**: `db` service has `shm_size: '256m'`. Without it, parallel hash aggregation queries fail with `"could not resize shared memory segment… No space left on device"` (SQLSTATE 53100).

- **Redis cache flush after format fixes**: If you fix a serialization format in the Go code, run `docker compose exec redis redis-cli FLUSHDB` to clear stale cached responses.

- **Translation job competition**: Local translators (`translator`, `translator-2`) and remote workers (WebSocket) compete via `FOR UPDATE SKIP LOCKED` on `traducciones`. Both must use the same `MAX_SEQ_PER_CALL` memory profile (**8 seq/call ≈ 1.8 GiB peak**; 16 → ~2.6 GiB, 32 → ~3.1 GiB, 128 → ~6.3 GiB — all OOM-killed by the cgroup with the 3G limit). The peak is dominated by the CTranslate2 KV cache: ~24 KB per token per sequence (12 layers × 1024 dims, int8), reserved up to `MAX_SRC_TOKENS + MAX_NEW_TOKENS`. Note the cgroup `memory` limit counts swapped pages too, so growing host swap does **not** help this limit — only reducing the container's peak does.
- **Host swap**: 8 GB total (`/swap.img` 4 GB + `/swap2.img` 4 GB, both in `/etc/fstab`). The host has 4 cores and is CPU-oversubscribed (load ~15), so the translators are CPU-starved, not memory-starved.
- **Log rotation is mandatory**: `docker-compose.yml` defines the `x-logging` anchor (`max-size: 20m`, `max-file: 3`) applied to every service. Without it a worker stuck in an error loop filled 23 GB in the Postgres log alone. Recreating a container is what applies it (`docker compose up -d --force-recreate <svc>`).
- **spaCy needs `preshed==3.0.13`**: `preshed` 3.0.14 breaks loading *any* spaCy model with `OverflowError: Python int too large to convert to C ssize_t` in `Tokenizer._flush_specials` (PreshMap iterates uint64 keys ≥ 2^63). Pinned in `Dockerfile.ner`. Upstream: https://github.com/explosion/spaCy/issues/14047. Symptom: the NER worker exits at startup with "No se pudo cargar ningún modelo spaCy".
- **Scheduler has two paths** (`workers/translation_scheduler.py`): fast path = noticias inserted in the last `SCHEDULER_LOOKBACK_MINUTES` (index `idx_noticias_created_at`, ~2 ms); catch-up path = noticias updated in the last `SCHEDULER_CATCHUP_LOOKBACK_HOURS` (index `idx_noticias_updated_at`), which catches news that `langdetect` just tagged (the `update_noticias_updated_at` trigger refreshes `updated_at` on every UPDATE). The old single anti-join scanned the whole catalogue and took ~12 min per cycle.
- **API `total` must respect filters**: `GetNews` counts with the same WHERE as the listing. Returning `reltuples` of `noticias` (a global approximation) breaks pagination when filtering by `country_id`/`category_id`; and the count must be computed on every page, not only page 1 (otherwise `total=0, total_pages=1` from page 2 onwards).
- **Noticias has no `feed_id`**: the link to the feed is only `fuente_nombre` (text), and several feeds share the same name with *different* `pais_id`. Never assume `noticias.pais_id` equals the feed's current `pais_id` (13% of rows drifted after feeds were reconfigured).
- **Scale reality**: the ingestor adds ~1,500 news/min (33k feeds polled) while 2 CPU translators drain ~13/min. The corpus cannot be fully translated on this host — `remote-worker/` (WebSocket) is the scaling path, plus the `gpu` profile if NVIDIA hardware appears. The translator orders by `n.fecha DESC`, so recent news is prioritised regardless of the backlog.
- **Feeds marked as `traducido` are not processed**: `feeds.traducido` (migration 59) is a hand-curated list of channels that already publish in the target language. The ingestor inserts their news with `lang = TARGET_LANGS[0]` straight away, so langdetect skips them (it only picks rows with `lang` NULL/empty) and the scheduler never creates a translation job for them (`n.lang != 'es'`). Do **not** reuse `feeds.idioma` for this: it is metadata declared by the feed and is sometimes wrong. Endpoints: `POST /api/feeds/:id/traducido` and `POST /api/feeds/traducido/bulk`.
- **Noticiario search filters translated news**: `GET /api/search` accepts `translated_only=true` (the Noticiario enables it by default) so the news offered for an audio bulletin already have a finished Spanish translation. The semantic path always filtered by `t.status='done'`.
- **"Translated" means "text already available in Spanish"**, not "has a row in `traducciones`": the filter is `(t.status='done' AND t.titulo_trad IS NOT NULL AND t.titulo_trad != n.titulo) OR n.lang = <idioma destino>`. News from the `traducido` channels never get a translation row (the scheduler skips them), so requiring the row would silently drop exactly the news that are already in Spanish and ready to narrate. Consequence: **the JOIN with `traducciones` must stay LEFT** in both `/api/search` and `/api/news` when this filter is active — with INNER JOIN the `n.lang` branch is unreachable.
- **Semantic search covers Spanish originals too**: `traduccion_embeddings` has a nullable `traduccion_id` and a `noticia_id` column (migration 60). The embeddings worker embeds the *original* text of the news already in the target language (`traduccion_id IS NULL`), and `SemanticSearch` unions both sources. Without it the semantic search only ever returned translated news.
- **Alert scanner baseline is pooled, not averaged**: the baseline for an entity is `SUM(mentions) / SUM(news of those hours) × news_of_the_reference_hour` — i.e. its overall share of the news, volume-weighted. It used to be `AVG(cnt/noticias)` per bucket, and a bucket with 9 news and a single mention (ratio 11%) dominated the average, inflating the baseline of every entity in that hour so no spike ever qualified: the scan always returned 0. The pooled form only changes entities that appear in ≥2 buckets, and it is the correct estimator of "expected mentions in the reference hour". The scanner needs ≥2 previous buckets per entity (`MinBuckets`), so it produces nothing until a few hours of tagged news exist.
- **Alert news lists are filtered by the spike hour**: the alert's `periodo` is the hour bucket the detector used (`date_trunc('hour', n.fecha)`), so `GET /entities/news?periodo=YYYY-MM-DD HH24:MI` filters to `[periodo, periodo+1h)`. Without it the list shows every mention of the concept (hundreds), which is not what made the alert fire. The Alertas page sends it by default and offers a toggle to see all of them. `periodo` is parsed in the three formats it arrives in (alerts API `YYYY-MM-DD HH24:MI`, RFC3339 and naive ISO).
- **Alerts are cleared, not rebuilt from old data**: the `alertas` table is aggregate state (entity + hour), not derived from `noticias` by FK, so after a corpus reset it must be truncated explicitly. `POST /api/admin/alerts/scan` forces a scan; otherwise it runs every `ALERTS_SCAN_INTERVAL_MIN` (default 15).
- **The stack survives a host reboot**: `docker` and `containerd` are enabled, every container has `restart: unless-stopped`, and both swap files are in `/etc/fstab`. `data/pgdata` is a bind mount on the root filesystem (not a separate mount), so Postgres can never come up with an empty PGDATA. Two things were needed for a clean cold start: (1) the API and the Go workers **wait for the database** (up to ~60 s) instead of exiting, otherwise they die and Docker restarts them with backoff — several restarts per boot; (2) **nginx must resolve the upstream dynamically** (`resolver 127.0.0.11` + a variable in `proxy_pass`), because a static `upstream backend:8080` is resolved once at startup and keeps the old IP if the container is recreated or changes it after a reboot — the symptom is a 404 from an unrelated service. With variables in `proxy_pass` the URI must be passed explicitly as `$request_uri`, otherwise nginx forwards only the literal prefix and everything 404s.
- **The `rss2-backend-run-*` replicas are not compose-managed**: they were created with `docker run` to scale the clustering worker. Docker restarts them at boot (`unless-stopped`), but `docker compose up/down` does not touch them — recreate them by hand if the image changes.
- **Feeds are found by text search and deleted with their news**: `GET /api/feeds?q=...` searches `nombre`, `descripcion` and `url` (ILIKE). `POST /api/feeds/bulk-delete` accepts `{ids:[...]}` or `{q:"..."}` and deletes the feeds **and all their news** (the `noticias.feed_id` FK cascades to traducciones, tags, embeddings and clusters). Before migration 61 the relation was only `noticias.fuente_nombre` (a TEXT column, not a key: 26.947 names for 33.672 feeds, so names are shared), which made a precise delete impossible and left orphan news behind.
- **`feed_stats` must be rebuilt after a corpus reset**: the counts are maintained by the `trigger_update_feed_stats` trigger, which fires on INSERT/DELETE but **not on TRUNCATE**. After the reset it still held the pre-reset numbers (3.038.649 vs 350.840 real), so the "News" column of the feeds table showed inflated figures. Rebuild: `TRUNCATE feed_stats; INSERT INTO feed_stats SELECT fuente_nombre, count(*) FROM noticias WHERE fuente_nombre <> '' GROUP BY 1;`
- **News ids are the normalised URL**: `generateID` hashes the URL after `normalizeURL` (no scheme, no `www.`, no trailing slash, no tracking params, sorted query). Without it the same article served as `https://www.x.com/a`, `https://x.com/a` and `http://www.x.com/a` produced three different ids → three news and three translation jobs (22.000 duplicates in 4.700 groups after the reset).
- **Positional parameters in SQL are fragile**: the semantic search computed the placeholder of the target language as `len(extraArgs)+1`, but the language is always the *first* argument (`$1`); with any filter the placeholder collided with the filter's own `$N` and the query failed with "operator does not exist: integer >= character". The Noticiario always sends `min_chars` and `desde`, so the semantic mode was completely broken. Rule: the target language is `$1`, the filters follow, and LIMIT/OFFSET are the last two.
- **`SECRET_KEY` is mandatory**: `config.Load()` panics without it (fail fast, on purpose). Tests that call `Load()` after `os.Clearenv()` must set it. The CORS default is no longer `*` but the two dev servers; a request from any other origin gets no `Access-Control-Allow-Origin` header.
- **No CI exists**: `make test` and `npm test` are the only safety net and they were red for a long time (nothing runs them automatically).
- **Bulk delete requires the confirmation on the server**: `POST /api/feeds/bulk-delete` answers **428** with the impact preview (`feeds_affected`, `noticias_affected`) unless the body has `confirmar: true`. The browser `confirm()` is not a protection for whoever calls the API directly, and one wrong call deletes thousands of channels plus their news.
- **`make dedupe-noticias`** runs `init-db/63-dedupe-noticias.sql`: it collapses the news that are the same article with a variant URL (keeping the **newest** copy, the one the current ingestor generates, so the next `INSERT ... ON CONFLICT` does not re-create it) and rebuilds `feed_stats`. It is idempotent. Grouping by title is **wrong** (24.275 rows, only 1.098 real duplicates); group by (feed, normalised URL).
- **The embeddings worker walks `noticias` with a cursor on `created_at`**: the news id is an md5 (not orderable) and without the cursor the query was a parallel seq scan of the whole table (2,1 s every 5 s). With `created_at > cursor` it costs ~18 ms.
- **The ingestor must survive a feed being deleted mid-flight**: `noticias.feed_id` is a FK `ON DELETE CASCADE`, so deleting a feed also deletes its news. If the insert then fails on that FK the whole batch is lost; the worker now detects the FK violation (SQLSTATE 23503) and reports that the feed is gone instead of logging a generic "batch insert error" and moving on.
- **init-db/ must be applicable from scratch — and it was not**: `47-pgvector-init.sh` re-applied every `.sql` file (the entrypoint had already applied them), so migrations 00-46 ran twice: the countries were seeded twice (390 rows for 195 countries, because `paises.nombre` had no UNIQUE) and `99_stable_keys.sql` then died with "duplicate key value violates unique constraint paises_iso2_key", aborting the whole init with `ON_ERROR_STOP=1` (the migrations after 47 were never applied). Fixed: the `.sh` only creates the `vector` extension, the extension is also created in `00-complete-schema.sql` (00/01/09/34 already use the `vector` type) and `paises.nombre` is UNIQUE. Verify with `make check-schema` and by applying init-db/ to a fresh `pgvector/pgvector:pg16` (the CI does exactly that).
- **`scripts/check-schema-drift.py` compares init-db/ with the live database**: tables, columns, indexes, triggers and functions declared in the scripts are looked up in the DB. It is what would have caught the missing migrations 51/52, the missing `update_noticias_updated_at` trigger and the `pg_cron` abort in migration 50. Run it with `make check-schema`.
- **The date a feed declares must be sanitised and normalised to UTC**: the ingestor applies `sanitizeDate` (rejects years < 1971 and anything more than 2 h in the future) and then `.UTC()`. Both are needed: `noticias.fecha` is `timestamp without time zone` and lib/pq sends the offset, which Postgres ignores for that type — so a wall-clock time was stored as if it were UTC. gofeed normalises `pubDate` to UTC but **not** `dc:date` (Dublin Core), so the feeds that use `dc:date` (Asahi, PR Times, tmz…) were stored up to 9 h off and appeared at the top of the home page. Without the sanitiser, a feed whose index page is dated 3000-01-01 also froze the scraper (see below).
- **Feeds may serve relative links**: some serve `/tour` or `//sitio.com/a`. The ingestor resolves them against the feed URL (`resolveURL`). Stored raw they were not downloadable (`unsupported protocol scheme`) and their id (md5 of the raw link) collided between feeds.
- **The scraper must not retry the same news forever**: `noticias.contenido IS NULL` used to mean "not enriched", so a failing URL came back on every tick — the worker re-tried the same 13 URLs 1.436 times each in 24 h and enriched 0 news. Now the attempts are tracked in `noticias_enrich_log` (a separate table on purpose: an UPDATE on `noticias` refreshes `updated_at` and would flood the translation scheduler's catch-up path), a news is skipped after `SCRAPER_ENRICH_MAX_ATTEMPTS` (3) for `SCRAPER_ENRICH_COOLDOWN_HOURS` (168), and the selection is `ORDER BY created_at DESC` (not `fecha`, which is feed data and can be corrupt).
- **The topics and langdetect workers lock `noticias` rows in id order**: langdetect updates row by row sorted by id; topics used to do a bulk `UPDATE ... FROM unnest(...)`, whose row locks are taken in plan order, so the two deadlocked (SQLSTATE 40P01) about 3 times a minute. Topics now pre-locks the batch with `SELECT id ... ORDER BY id FOR UPDATE` inside a transaction. **Any worker that UPDATEs `noticias` must take its locks in id ASC order.**
- **`entity_stats` is refreshed by the backend, not by pg_cron**: `pg_cron` is not in the `pgvector/pgvector:pg16` image and the `cron.schedule` call in migration 50 is commented out, so the table was only refreshed on startup (or via the admin endpoint) and served stale counts. `StartEntityStatsRefresher` now refreshes it every `ENTITY_STATS_REFRESH_INTERVAL_MIN` (60). When checking "is a refresh already running", exclude your own pid: the probe query contains the string `refresh_entity_stats`, so without `pid <> pg_backend_pid()` the check matches itself and always answers "yes".

## Resetting the news corpus

The news corpus can be rebuilt from the feeds (RSS only exposes a rolling window of ~10-50 items per feed, so a reset trades the 2.7M bulk-imported rows for ~500K fresh ones — the bulk 49% was 342 spam feeds: Wikipedia edit logs, blogspam, webcams). Procedure used on 2026-10-25:

1. Stop the workers, `pg_dump -Fc -Z 6` to `data/backup/` and **verify the dump restores** (`pg_restore --schema-only` + one table).
2. `TRUNCATE` the news and all derived state (`noticias`, `traducciones`, `traducciones_target`, `tags`, `tags_noticia`, `traduccion_embeddings`, `entity_*`, `event_clusters`, `eventos`, `noticia_clusters`, `related_noticias`, `news_topics`, `favoritos`, `tts_*`, `translation_stats`) `RESTART IDENTITY CASCADE`. Keep `feeds`, `fuentes_url`, `paises`, `categorias`, `idiomas_disponibles`, `users`, `user_*`, `config`.
3. Restart the chain in order: ingestor → langdetect → scheduler → translator → ner → topics → embeddings → enrichment (wiki, related, scraper, discovery, cooccurrences).
4. After recreating the `backend`, **restart nginx**: it resolves `upstream backend` once at startup and keeps the old IP, which may be recycled by another container (symptom: 404 from an unrelated service).

Note that `init-db/*.sql` only runs on the **first** initialisation of `data/pgdata`. Migrations added later are never applied automatically — check for missing columns/tables before rebuilding any image that uses them (e.g. `traducciones.confidence`/`retry_count` from migration 51 were missing and would have broken every translation UPDATE).

## Sources of Truth ( Constitution)

When docs conflict with code or config, trust in this order:
1. **Schema**: `init-db/*.sql` (e.g., `noticias.id VARCHAR(32)`, no `contenido` column historically)
2. **docker-compose.yml**: services, networks, env vars — what has no service does not run
3. **HTTP contracts**: `backend/cmd/server/main.go` + `backend/internal/models/` + `frontend/src/services/api.ts`
4. **specs/**: agreed behavior documentation

## Important DB Conventions

- **Never delete `data/pgdata`** in production — it contains the Postgres data directory
- **Rotate passwords**: use `ALTER USER` via SQL, then update `.env`; never rely on init script re-run
- **Sanitize restore**: `make sanitize-restore` reads from `noticias.*_raw` columns, not backups
- **TTS schema duplication**: the DDL of the `tts_*` tables exists in
  `init-db/57-add-tts.sql`, `migrations/add_tts.sql` and `workers/tts/schema.py`.
  They must stay in sync — `workers/tests/test_tts_schema_parity.py` checks it
  (run it with `make tts-test`).
- **TTS service is internal**: it publishes no ports and is only reachable on
  the `backend` network. The Go backend is the only public entry point, and
  audio is only served to authenticated users.

## Monorepo Structure

| Directory | Language | Role |
|-----------|----------|------|
| `backend/` | Go | API server + Go workers (scraper, discovery, topics, wiki, related, sanitize, entityscan) |
| `rss-ingestor-go/` | Go | RSS feed ingestion (separate module) |
| `workers/` | Python | ner, langdetect, translation-scheduler, embeddings, tts |
| `translator/` | Python | ctranslator worker (NLLB via CTranslate2) |
| `frontend/` | TypeScript/React | SPA (Vite) |
| `init-db/` | SQL | Postgres schema + migrations (numbered) |
| `specs/` | Markdown | Architecture docs, specs, constitution |

## Testing

- **Go**: `make test` runs `go test ./...` in both `backend/` and `rss-ingestor-go/`
- **Frontend**: `npm test` (vitest) in `frontend/`
- **TTS**: `make tts-test` (pytest en `workers/tests`); las pruebas de integración
  necesitan `TEST_DATABASE_URL` (`make tts-test-db` crea `rss_tts_test`).
  El target corre las pruebas **en un contenedor** (`Dockerfile.tts.test`, que
  extiende la imagen del servicio con pytest): en el host fallaban con
  "No module named soundfile" (el host tiene Python 3.14 y las versiones
  fijadas no tienen ruedas para esa versión). `make tts-test-local` es la
  variante con un venv propio.
- **Integration**: Test against live stack with `docker compose`; no CI pipelines found in repo

## Remote Workers

Remote translation workers connect via WebSocket to `backend:8080/ws/worker` using an API key from the `remote_workers` table. See `remote-worker/README.md` for deployment details.
