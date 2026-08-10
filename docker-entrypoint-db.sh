#!/bin/bash
set -e

# Directorio de datos de PostgreSQL
PGDATA_DIR="/var/lib/postgresql/data/18/main"

echo "RSS2: Checking database presence..."

# REGLA: nunca se borra ni se modifica el directorio de datos automáticamente.
# Solo se informa del estado y se deja que el entrypoint oficial de PostgreSQL
# inicialice (si no existe) o arranque con los datos existentes.
if [ -f "$PGDATA_DIR/PG_VERSION" ]; then
    echo "RSS2: Existing database found at $PGDATA_DIR - starting normally"
else
    echo "RSS2: No database found at $PGDATA_DIR - docker-entrypoint will initialize it"
fi

# Ejecutar el entrypoint original con los parámetros de PostgreSQL.
# Si la base de datos estuviera corrupta, PostgreSQL fallará al arrancar y
# registrará el error en los logs; NO se elimina ningún dato de forma automática.
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
