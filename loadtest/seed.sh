#!/usr/bin/env bash
# Loads the k6 load-test seed data into the running delta-postgres container.
#
# Usage:
#   ./loadtest/seed.sh
#
# Requires the docker compose stack to be up. Postgres credentials default to
# the compose defaults; override with POSTGRES_USER / POSTGRES_DB if changed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-delta}"

docker exec -i delta-postgres \
  psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
  < "${SCRIPT_DIR}/seed.sql"

echo "Load-test seed data applied."
