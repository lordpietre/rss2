#!/bin/bash
set -e

# Detectar si la base de datos necesita reinicialización
PGDATA_DIR="/var/lib/postgresql/data/18/main"

echo "RSS2: Checking database integrity..."

# Si no existe el archivo de versión, es una base de datos nueva
if [ ! -f "$PGDATA_DIR/PG_VERSION" ]; then
    echo "RSS2: New database - will be initialized by docker-entrypoint"
else
    # Verificar si la base de datos es funcional
    if ! pg_isready -h localhost -p 5432 -U "${POSTGRES_USER:-rss}" 2>/dev/null; then
        echo "RSS2: Database appears corrupted - removing old data files for fresh initialization..."
        # Eliminar solo los archivos de datos, no todo el directorio
        rm -rf "$PGDATA_DIR"/*
        echo "RSS2: Data files removed - docker-entrypoint will initialize fresh database"
    else
        echo "RSS2: Database is healthy"
    fi
fi

# Ejecutar el entrypoint original con los parámetros de PostgreSQL
exec docker-entrypoint.sh \
    postgres \
    -c max_connections=200 \
    -c shared_buffers=4GB \
    -c effective_cache_size=12GB \
    -c work_mem=16MB \
    -c maintenance_work_mem=512MB \
    -c autovacuum_max_workers=3 \
    -c autovacuum_vacuum_scale_factor=0.02 \
    -c autovacuum_vacuum_cost_limit=1000 \
    -c max_worker_processes=8 \
    -c max_parallel_workers=6 \
    -c max_parallel_workers_per_gather=2 \
    -c wal_level=replica \
    -c max_wal_senders=5 \
    -c wal_keep_size=1GB \
    -c hot_standby=on \
    "$@"
