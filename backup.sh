#!/bin/bash
# Backup diario de PostgreSQL rss2
set -e
set -o pipefail  # sin esto, un pg_dump fallido queda enmascarado por gzip

BACKUP_DIR="/home/x/rss2/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUT="$BACKUP_DIR/rss-${TIMESTAMP}.sql.gz"
LOG="$BACKUP_DIR/backup.log"

mkdir -p "$BACKUP_DIR"
docker exec rss2_db pg_dump -U rss -d rss --no-owner --no-privileges | gzip > "$OUT"

# Dumps fallidos (pg_dump sin conexión escribe ~20 bytes): no conservarlos
if [ "$(stat -c%s "$OUT")" -lt 1024 ]; then
  echo "$(date '+%F %T') Backup FALLIDO (fichero diminuto, eliminado): $OUT" >> "$LOG"
  rm -f "$OUT"
  exit 1
fi

# Retener solo los ultimos 30 dias (borrado seguro de ficheros concretos, nunca rm -rf)
find "$BACKUP_DIR" -maxdepth 1 -name 'rss-*.sql.gz' -mtime +30 -delete

echo "$(date '+%F %T') Backup OK: $OUT ($(du -h "$OUT" | cut -f1))" >> "$LOG"
