#!/usr/bin/env bash

# Local-only helper to run a DFBA+BEAST longrun on the minimal 4-node cluster.
# This script does not modify metrics/log dimensions and is not wired into CI.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${ROOT_DIR}"

DOCKER_COMPOSE_BIN="${DOCKER_COMPOSE:-docker compose}"
RUNTIME_MIN="${RUNTIME_MIN:-120}"

echo "[dfba-beast-longrun] using compose: ${DOCKER_COMPOSE_BIN}"
echo "[dfba-beast-longrun] runtime (minutes): ${RUNTIME_MIN}"

${DOCKER_COMPOSE_BIN} up --build -d

trap 'echo "[dfba-beast-longrun] stopping cluster"; ${DOCKER_COMPOSE_BIN} down -v || true' EXIT

echo "[dfba-beast-longrun] waiting for cluster to warm up (60s)"
sleep 60

echo "[dfba-beast-longrun] running m4 health check"
WAIT_SECS=60 bash ./.github/scripts/m4-health-check.sh || true

echo "[dfba-beast-longrun] enabling BEAST attacker mode (ATTACK_BEAST=1)"
docker exec adversary-agent /bin/sh -c 'export ATTACK_BEAST=1; echo "ATTACK_BEAST=1" > /tmp/attacker.env' || true

TOTAL_SECS=$((RUNTIME_MIN * 60))
STEP=60
ELAPSED=0
while [ "${ELAPSED}" -lt "${TOTAL_SECS}" ]; do
  echo "[dfba-beast-longrun] tick ${ELAPSED}/${TOTAL_SECS}s"
  sleep "${STEP}"
  ELAPSED=$((ELAPSED + STEP))
done

echo "[dfba-beast-longrun] finished runtime window"

