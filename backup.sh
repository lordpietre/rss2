#!/bin/bash
# Backup diario de PostgreSQL rss2
set -e

BACKUP_DIR="/home/x/rss2/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUT="$BACKUP_DIR/rss-${TIMESTAMP}.sql.gz"
LOG="$BACKUP_DIR/backup.log"

mkdir -p "$BACKUP_DIR"
docker exec rss2_db pg_dump -U rss -d rss --no-owner --no-privileges | gzip > "$OUT"

# Retener solo los ultimos 30 dias (borrado seguro de ficheros concretos, nunca rm -rf)
find "$BACKUP_DIR" -maxdepth 1 -name 'rss-*.sql.gz' -mtime +30 -delete

echo "$(date '+%F %T') Backup OK: $OUT ($(du -h "$OUT" | cut -f1))" >> "$LOG"
