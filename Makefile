# RSS2 Workers Makefile

.PHONY: all build clean deps ingestor scraper discovery topics related qdrant server

# Binary output directory
BIN_DIR := bin

# Main binaries
SERVER := $(BIN_DIR)/server
INGESTOR := $(BIN_DIR)/rss-ingestor
SCRAPER := $(BIN_DIR)/scraper
DISCOVERY := $(BIN_DIR)/discovery
TOPICS := $(BIN_DIR)/topics
RELATED := $(BIN_DIR)/related
QDRANT := $(BIN_DIR)/qdrant-worker

all: deps build

deps:
	cd backend && go mod download
	cd backend && go mod tidy

# Build all workers
build: ingestor scraper discovery topics related qdrant server

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

qdrant:
	cd backend && go build -o $(QDRANT) ./cmd/qdrant

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

run-qdrant:
	DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss \
		QDRANT_HOST=localhost QDRANT_PORT=6333 OLLAMA_URL=http://localhost:11434 $(QDRANT)

# Docker builds
docker-build:
	docker build -t rss2-ingestor -f rss-ingestor-go/Dockerfile ./rss-ingestor-go
	docker build -t rss2-server -f backend/Dockerfile ./backend
	docker build -t rss2-scraper -f Dockerfile.scraper ./backend
	docker build -t rss2-discovery -f Dockerfile.discovery ./backend
	docker build -t rss2-topics -f Dockerfile.topics ./backend
	docker build -t rss2-related -f Dockerfile.related ./backend
	docker build -t rss2-qdrant -f Dockerfile.qdrant ./backend
	docker build -t rss2-langdetect -f Dockerfile .
	docker build -t rss2-scheduler -f Dockerfile.scheduler .
	docker build -t rss2-translator -f Dockerfile.translator .
	docker build -t rss2-translator-gpu -f Dockerfile.translator-gpu .
	docker build -t rss2-embeddings -f Dockerfile.embeddings_worker .
	docker build -t rss2-ner -f Dockerfile .
	docker build -t rss2-llm-categorizer -f Dockerfile.llm_worker .
	docker build -t rss2-frontend -f frontend/Dockerfile ./frontend
	docker build -t rss2-nginx -f Dockerfile.nginx .
