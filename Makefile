# RSS2 Workers Makefile

.PHONY: all build clean deps ingestor scraper discovery topics related server test parity sanitize sanitize-dry-run sanitize-restore sanitize-local entities-scan entities-scan-apply entities-scan-local alerts-prune alerts-prune-apply alerts-prune-local dedupe-noticias event-clustering tts-models tts-test-db tts-test tts-test-local

# Binary output directory
BIN_DIR := bin

# Main binaries
SERVER := $(BIN_DIR)/server
INGESTOR := $(BIN_DIR)/rss-ingestor
SCRAPER := $(BIN_DIR)/scraper
DISCOVERY := $(BIN_DIR)/discovery
TOPICS := $(BIN_DIR)/topics
RELATED := $(BIN_DIR)/related
EVENT_CLUSTERING := $(BIN_DIR)/event_clustering

all: deps build

deps:
	cd backend && go mod download
	cd backend && go mod tidy

# Build all workers
build: ingestor scraper discovery topics related event-clustering server

# Ingestor
ingestor:
	cd rss-ingestor-go && go build -o ../$(INGESTOR) .

# Server
server:
	cd backend && go build -o $(SERVER) ./cmd/server

# Workers
scraper:
	cd backend && go build -o $(SCRAPER) ./cmd/scraper

discovery:
	cd backend && go build -o $(DISCOVERY) ./cmd/discovery

topics:
	cd backend && go build -o $(TOPICS) ./cmd/topics

related:
	cd backend && go build -o $(RELATED) ./cmd/related

event-clustering:
	cd backend && go build -o $(EVENT_CLUSTERING) ./cmd/event_clustering

# Clean
clean:
	rm -rf $(BIN_DIR)
	cd backend && go clean

# Tests Go (los dos módulos)
test:
	cd backend && go test ./...
	cd rss-ingestor-go && go test ./...

# Tests del frontend. Se ejecutan dentro de la imagen node:20-alpine para no
# depender de Node instalado en el host. El tmpfs sobre node_modules/.vite
# evita que el contenedor (que corre con el usuario actual) intente escribir en
# un directorio de node_modules propiedad de root.
#
# Requiere node_modules ya instalado en frontend/ (npm install dentro del
# contenedor si no lo está: `make frontend-deps`).
test-frontend:
	docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp \
		--tmpfs /app/node_modules/.vite:rw,uid=$$(id -u),gid=$$(id -g) \
		-v $(PWD)/frontend:/app -w /app node:20-alpine npx vitest run

# Instala las dependencias del frontend dentro del contenedor (sin tocar el host).
frontend-deps:
	docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp \
		--tmpfs /app/node_modules:rw,uid=$$(id -u),gid=$$(id -g) \
		-v $(PWD)/frontend:/app -w /app node:20-alpine npm install

# textclean está duplicado a propósito: ingestor y backend se construyen con
# contextos de Docker separados. Este target garantiza que no haya deriva.
parity:
	diff -r backend/internal/textclean rss-ingestor-go/textclean
	@echo "textclean: ambas copias idénticas"

# Backfill de limpieza (HTML y ruido) sobre las noticias ya existentes.
# Se ejecuta en el contenedor backend (tiene la BD y el binario /sanitize):
#   dry-run no escribe nada; `sanitize` aplica + re-traduce lo que cambió;
#   `sanitize-restore` deshace desde noticias.*_raw.
sanitize-dry-run:
	docker compose run --rm --entrypoint /sanitize backend

sanitize:
	docker compose run --rm --entrypoint /sanitize backend -apply -requeue

sanitize-restore:
	docker compose run --rm --entrypoint /sanitize backend -restore -apply -requeue

# Variante local (requiere Go y acceso directo a la BD)
sanitize-local:
	cd backend && go run ./cmd/sanitize -apply -requeue

# Falsos positivos del NER (entidades/temas de TODOS los países y tipos).
# Dry-run informa; -apply siembra entity_blocklist, que filtran la API y que
# ner_worker ya no vuelve a insertar.
entities-scan:
	docker compose run --rm --entrypoint /entityscan backend

entities-scan-apply:
	docker compose run --rm --entrypoint /entityscan backend -apply

entities-scan-local:
	cd backend && go run ./cmd/entityscan -apply

# Podado de alertas heredadas: marca como `descartada` las que no cumplen los
# umbrales racionalizados (spec 04), la blocklist o las reglas FP fuertes.
alerts-prune:
	docker compose run --rm --entrypoint /entityscan backend -alertas

alerts-prune-apply:
	docker compose run --rm --entrypoint /entityscan backend -alertas -apply

alerts-prune-local:
	cd backend && go run ./cmd/entityscan -alertas -apply

# Elimina noticias duplicadas (mismo articulo con variantes de URL) y
# regenera los contadores por fuente. Es idempotente: se puede repetir.
dedupe-noticias:
	docker compose exec -T db psql -v ON_ERROR_STOP=1 -U $${POSTGRES_USER:-rss} -d $${POSTGRES_DB:-rss} \
		-f /docker-entrypoint-initdb.d/63-dedupe-noticias.sql

# Comprueba que la base tiene todo lo que declaran los scripts de init-db/.
# init-db/ solo se ejecuta en la PRIMERA inicializacion: una migracion
# anadida despues no se aplica nunca y el sistema se rompe en silencio.
check-schema:
	DB_PASS=$${POSTGRES_PASSWORD} python3 scripts/check-schema-drift.py

# Run workers locally (requires DB and services)
run-scraper:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss $(SCRAPER)

run-discovery:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss $(DISCOVERY)

run-topics:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss $(TOPICS)

run-related:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss RELATED_SLEEP=10 $(RELATED)

run-event-clustering:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss CLUSTERING_SLEEP=900 CLUSTERING_BATCH=500 $(EVENT_CLUSTERING)

# ---------------------------------------------------------------------------
# Subsistema de sintesis de voz (TTS)
# ---------------------------------------------------------------------------
# Intérprete para las pruebas de Python. Debe tener las dependencias de
# workers/requirements-tts.txt (numpy, soundfile, flask, psycopg2-binary,
# redis, pytest). Por defecto, el del entorno actual.
PYTHON ?= python3

# Prepara los pesos (Chatterbox V3 + OpenVoice V2 + voz espanola de MeloTTS)
# en los volumenes ./models/chatterbox y ./models/openvoice. No se ejecuta al
# arrancar el servicio.
tts-models:
	./scripts/prepare_tts_models.sh

# Base de datos de pruebas del subsistema TTS (rss_tts_test).
tts-test-db:
	./scripts/setup_tts_test_db.sh

# Pruebas del subsistema TTS. Las unitarias no necesitan modelo ni pesos.
# Las de integracion necesitan TEST_DATABASE_URL (ver `make tts-test-db`).
#
# Se ejecutan en un contenedor: la imagen del servicio ya trae las
# dependencias (soundfile, librosa, psycopg2) y solo anade pytest. En el
# host fallaban con "No module named soundfile" (el host tiene Python
# 3.14 y las versiones fijadas no tienen ruedas para esa version).
tts-test:
	docker compose build tts
	docker build -f Dockerfile.tts.test -t rss2-tts-test .
	docker run --rm --network rss2_backend \
		-e TEST_DATABASE_URL="$${TEST_DATABASE_URL:-}" \
		rss2-tts-test

# Variante local: requiere un entorno con workers/requirements-tts.txt
# instalado (python -m venv .venv-tts && . .venv-tts/bin/activate).
tts-test-local:
	$(PYTHON) -m pytest workers/tests -v

# ---------------------------------------------------------------------------
# Docker builds (vía compose: los Dockerfiles legacy de raíz se eliminaron;
# los workers Go reutilizan la imagen backend, ver docker-compose.yml)
# ---------------------------------------------------------------------------
docker-build:
	docker compose build backend frontend ingestor translator
	docker compose build translation-scheduler langdetect ner embeddings
	docker compose build tts
