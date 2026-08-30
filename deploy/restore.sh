#!/usr/bin/env bash
# Restore a delta backup into the running postgres container.
#
# Usage:
#   ./deploy/restore.sh /var/backups/delta/delta-20260101T020000Z.sql.gz
#
# WARNING: this overwrites the current database contents.
set -euo pipefail

FILE="${1:?usage: restore.sh <backup.sql.gz>}"
DB_CONTAINER="${DB_CONTAINER:-delta-postgres}"
DB_NAME="${DB_NAME:-delta}"
DB_USER="${DB_USER:-postgres}"

gunzip -c "$FILE" | docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME"
echo "restored from $FILE"