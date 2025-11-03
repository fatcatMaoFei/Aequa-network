#!/usr/bin/env sh
set -eu

# rolling_upgrade.sh — serially restart services from a compose file.
# Usage:
#   COMPOSE="docker compose" ./rolling_upgrade.sh -f deploy/testnet/docker-compose.yml node-0 node-1 ...
# Options:
#   -f, --file       compose file path (required)
#   -w, --wait       seconds to wait after each restart (default 5)
#   -h, --help       print help

COMPOSE_BIN=${COMPOSE:-docker compose}
FILE=""
WAIT_SECS=5
SERVICES=""

usage() {
  echo "usage: $0 -f <compose.yml> [-w seconds] service [service ...]" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -f|--file) FILE="$2"; shift 2 ;;
    -w|--wait) WAIT_SECS="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) SERVICES="$SERVICES $1"; shift ;;
  esac
done

if [ -z "$FILE" ] || [ -z "$SERVICES" ]; then
  usage; exit 2
fi

for s in $SERVICES; do
  echo ">> rolling: $s"
  # stop may fail if not running; ignore
  $COMPOSE_BIN -f "$FILE" stop "$s" >/dev/null 2>&1 || true
  $COMPOSE_BIN -f "$FILE" up -d "$s"
  echo "   waiting ${WAIT_SECS}s for $s"
  sleep "$WAIT_SECS"
done

echo ">> done"

