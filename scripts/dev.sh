#!/usr/bin/env bash
# Run the API locally with hot-reload via air.
# Requires: docker compose up db embedder (for postgres + embedder sidecar)
#
# Usage: ./scripts/dev.sh [air flags]

set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "ERROR: .env not found. Copy .env.example and fill in API keys." >&2
  exit 1
fi

# Load all vars from .env into the current shell
set -a
# shellcheck disable=SC1091
source .env
set +a

# Override docker-internal service names with localhost equivalents.
# DB is mapped to localhost:5434 and embedder to localhost:8001 in docker-compose.yml.
export DB_URL="postgres://postgres:postgres@localhost:5434/postgres?sslmode=disable"
export EMBEDDER_URL="http://localhost:8001"
export PORT="${PORT:-8080}"

echo "→ DB:       $DB_URL"
echo "→ Embedder: $EMBEDDER_URL"
echo "→ Port:     $PORT"
echo ""

exec air "$@"
