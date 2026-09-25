# RSS2 Workers Makefile

.PHONY: all build clean deps ingestor scraper discovery topics related server

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
