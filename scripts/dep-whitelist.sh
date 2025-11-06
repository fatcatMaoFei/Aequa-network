#!/usr/bin/env bash
set -euo pipefail
# dep-whitelist.sh — validate go.mod dependencies against an allowlist.
# Passes when there is no `require` block or all modules are approved.

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

ALLOW=(
  "github.com/supranational/blst"
)

if [ ! -f go.mod ]; then
  echo "ok: no go.mod"; exit 0
fi

REQ=$(grep -E '^\s*require\s+\(' -n go.mod || true)
if [ -z "$REQ" ]; then
  # also handle single-line requires
  MODS=$(grep -E '^[[:space:]]*require[[:space:]]+[^\(]+' go.mod | awk '{print $2}' || true)
else
  MODS=$(awk '/require \(/{flag=1;next}/\)/{flag=0}flag' go.mod | awk '{print $1}' || true)
fi

if [ -z "$MODS" ]; then
  echo "ok: no external modules"; exit 0
fi

ok=true
for m in $MODS; do
  found=false
  for a in "${ALLOW[@]}"; do
    if [ "$m" = "$a" ]; then found=true; break; fi
  done
  if [ "$found" != true ]; then
    echo "error: module '$m' not in whitelist" >&2
    ok=false
  fi
done

$ok || exit 1
echo "ok: dependencies whitelisted"