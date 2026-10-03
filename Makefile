# RSS2 Workers Makefile

.PHONY: all build clean deps ingestor scraper discovery topics related server test parity sanitize sanitize-dry-run sanitize-restore sanitize-local entities-scan entities-scan-apply entities-scan-local alerts-prune alerts-prune-apply alerts-prune-local

# Binary output directory
BIN_DIR := bin

# Main binaries
SERVER := $(BIN_DIR)/server
INGESTOR := $(BIN_DIR)/rss-ingestor
SCRAPER := $(BIN_DIR)/scraper
DISCOVERY := $(BIN_DIR)/discovery
TOPICS := $(BIN_DIR)/topics
RELATED := $(BIN_DIR)/related

all: deps build

deps:
	cd backend && go mod download
	cd backend && go mod tidy

# Build all workers
build: ingestor scraper discovery topics related server

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

# Clean
clean:
	rm -rf $(BIN_DIR)
	cd backend && go clean

# Tests Go (los dos módulos)
test:
	cd backend && go test ./...
	cd rss-ingestor-go && go test ./...

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

# Run workers locally (requires DB and services)
run-scraper:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss $(SCRAPER)

run-discovery:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss $(DISCOVERY)

run-topics:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss $(TOPICS)

run-related:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss RELATED_SLEEP=10 $(RELATED)

# Docker builds (vía compose: los Dockerfiles legacy de raíz se eliminaron;
# los workers Go reutilizan la imagen backend, ver docker-compose.yml)
docker-build:
	docker compose build backend frontend ingestor translator
	docker compose build translation-scheduler langdetect ner embeddings
