#!/bin/bash

echo "Stopping all containers..."
docker-compose down

echo "Removing data volumes..."
# Use sudo if necessary, or ensure current user has permissions
rm -rf pgdata pgdata-replica redis-data qdrant_storage

echo "Starting deployment from scratch..."
docker-compose up -d --build

echo "Deployment complete. Checking status..."
docker-compose ps
