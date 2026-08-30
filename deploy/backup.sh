#!/usr/bin/env bash
# Daily Postgres backup for the delta database.
#
# Usage (as root or with sudo):
#   ./deploy/backup.sh
#
# Configuration via environment variables:
#   DB_CONTAINER  (default delta-postgres)
#   DB_NAME       (default delta)
#   DB_USER       (default postgres)
#   BACKUP_DIR    (default /var/backups/delta)
#   KEEP_DAYS     (default 14)
#   RCLONE_REMOTE (optional; e.g. b2:delta-backups to also upload off-site)
#
# Cron example (run daily at 02:00):
#   0 2 * * * /root/delta/deploy/backup.sh >> /var/log/delta-backup.log 2>&1
set -euo pipefail

DB_CONTAINER="${DB_CONTAINER:-delta-postgres}"
DB_NAME="${DB_NAME:-delta}"
DB_USER="${DB_USER:-postgres}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/delta}"
KEEP_DAYS="${KEEP_DAYS:-14}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"

mkdir -p "$BACKUP_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$BACKUP_DIR/delta-$STAMP.sql.gz"

docker exec "$DB_CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" | gzip > "$OUT"
echo "backup written: $OUT ($(du -h "$OUT" | cut -f1))"

if [ -n "$RCLONE_REMOTE" ]; then
	rclone copy "$OUT" "$RCLONE_REMOTE" --b2-chunk-size 5M
	echo "uploaded to $RCLONE_REMOTE"
fi

find "$BACKUP_DIR" -name 'delta-*.sql.gz' -mtime +"$KEEP_DAYS" -delete
echo "pruned backups older than ${KEEP_DAYS} days"