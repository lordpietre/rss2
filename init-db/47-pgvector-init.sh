#!/bin/bash
# Crea la extension pgvector en la base de datos.
#
# IMPORTANTE: este script NO debe volver a aplicar los ficheros .sql. El
# entrypoint de Postgres ya los aplica todos, en orden, y este script se
# ejecuta EN MEDIO de esa lista: al reaplicarlos se ejecutaban dos veces
# (00..46) y todo se duplicaba (los paises se sembraban 2 veces -> 390 filas
# para 195 paises -> la asignacion de codigos ISO de la migracion 99
# reventaba con "duplicate key value violates unique constraint
# paises_iso2_key" y la inicializacion abortaba).
#
# La extension se crea tambien en 00-complete-schema.sql (la primera
# migracion), porque 00, 01, 09 y 34 ya usan el tipo vector. Aqui queda
# como red de seguridad por si el orden de ejecucion cambia.

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
