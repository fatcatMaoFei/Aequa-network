#!/usr/bin/env sh
set -eu

# rollback_all.sh — stop and start all services from a compose file.
# Intended for quick rollback when the previous image tag has been re-applied.
# Usage:
#   ./rollback_all.sh deploy/testnet/docker-compose.yml

COMPOSE_BIN=${COMPOSE:-docker compose}

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <compose.yml>" >&2
  exit 2
fi

FILE="$1"

$COMPOSE_BIN -f "$FILE" stop || true
$COMPOSE_BIN -f "$FILE" up -d

echo ">> rollback cycle completed"

