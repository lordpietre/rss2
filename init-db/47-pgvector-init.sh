#!/bin/bash
# Script para inicializar pgvector después de que PostgreSQL esté listo

set -e

echo "Waiting for PostgreSQL to be ready..."
until PGPASSWORD=$POSTGRES_PASSWORD psql -h "$DB_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\q' 2>/dev/null; do
    echo "PostgreSQL is unavailable - sleeping"
    sleep 1
done

echo "PostgreSQL is up - creating vector extension..."

PGPASSWORD=$POSTGRES_PASSWORD psql -h "$DB_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS vector CASCADE;
EOSQL

echo "Vector extension created successfully"

# Run the rest of the init scripts if they exist
for f in /docker-entrypoint-initdb.d/*.sql; do
    case "$f" in
        *pgvector*) echo "Skipping $f (already processed)" ;;
        *) echo "Running $f"; PGPASSWORD=$POSTGRES_PASSWORD psql -h "$DB_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$f" ;;
    esac
done

echo "Initialization complete"
