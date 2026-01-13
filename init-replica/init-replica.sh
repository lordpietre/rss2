#!/bin/bash
# Initialization script for PostgreSQL streaming replica
# This script sets up the replica from the primary using pg_basebackup

set -e

PGDATA="${PGDATA:-/var/lib/postgresql/data/18/main}"
PRIMARY_HOST="${PRIMARY_HOST:-db}"
REPLICATION_USER="${REPLICATION_USER:-replicator}"
REPLICATION_PASSWORD="${REPLICATION_PASSWORD:-replica_password}"

echo "=== PostgreSQL Replica Initialization ==="

# Check if PGDATA already has data (replica already initialized)
if [ -f "$PGDATA/standby.signal" ]; then
    echo "Replica already initialized (standby.signal exists). Skipping initialization."
    exit 0
fi

if [ -f "$PGDATA/PG_VERSION" ]; then
    echo "PGDATA already contains data. Checking if it's a replica..."
    if [ -f "$PGDATA/standby.signal" ] || grep -q "primary_conninfo" "$PGDATA/postgresql.auto.conf" 2>/dev/null; then
        echo "Already configured as replica. Skipping."
        exit 0
    else
        echo "WARNING: PGDATA contains data but is NOT a replica."
        echo "Cleaning up existing data to initialize replica..."
        rm -rf "$PGDATA"/*
        # Continue to basebackup
    fi
fi

echo "Waiting for primary at $PRIMARY_HOST to be ready..."
until pg_isready -h "$PRIMARY_HOST" -p 5432 -U postgres; do
    echo "Primary not ready yet. Waiting 2 seconds..."
    sleep 2
done

echo "Primary is ready. Starting pg_basebackup..."

# Use pg_basebackup to copy data from primary
PGPASSWORD="$REPLICATION_PASSWORD" pg_basebackup \
    -h "$PRIMARY_HOST" \
    -p 5432 \
    -U "$REPLICATION_USER" \
    -D "$PGDATA" \
    -Fp \
    -Xs \
    -P \
    -R

echo "pg_basebackup complete. Replica initialized successfully."
echo "standby.signal and postgresql.auto.conf with primary_conninfo created."
