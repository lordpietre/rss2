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

- **Translation job competition**: Local translators (`translator`, `translator-2`) and remote workers (WebSocket) compete via `FOR UPDATE SKIP LOCKED` on `traducciones`. Both must use the same `MAX_SEQ_PER_CALL` memory profile (32 seq/call ≈ 2 GiB peak; 128 caused OOM kills).

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

## Monorepo Structure

| Directory | Language | Role |
|-----------|----------|------|
| `backend/` | Go | API server + Go workers (scraper, discovery, topics, wiki, related, sanitize, entityscan) |
| `rss-ingestor-go/` | Go | RSS feed ingestion (separate module) |
| `workers/` | Python | ner, langdetect, translation-scheduler, embeddings |
| `translator/` | Python | ctranslator worker (NLLB via CTranslate2) |
| `frontend/` | TypeScript/React | SPA (Vite) |
| `init-db/` | SQL | Postgres schema + migrations (numbered) |
| `specs/` | Markdown | Architecture docs, specs, constitution |

## Testing

- **Go**: `make test` runs `go test ./...` in both `backend/` and `rss-ingestor-go/`
- **Frontend**: `npm test` (vitest) in `frontend/`
- **Integration**: Test against live stack with `docker compose`; no CI pipelines found in repo

## Remote Workers

Remote translation workers connect via WebSocket to `backend:8080/ws/worker` using an API key from the `remote_workers` table. See `remote-worker/README.md` for deployment details.
