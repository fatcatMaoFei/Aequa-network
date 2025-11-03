#!/usr/bin/env sh
set -eu

# kpi_active.sh — count active nodes by scraping /metrics for dvt_up 1
# Usage:
#   ./kpi_active.sh endpoints.txt
#   ./kpi_active.sh host1:4620 host2:4620 ...

if [ "$#" -eq 0 ]; then
  echo "usage: $0 endpoints.txt | host:port [host:port ...]" >&2
  exit 2
fi

list=""
if [ "$#" -eq 1 ] && [ -f "$1" ]; then
  list=$(grep -v '^[# ]' "$1" | tr '\n' ' ')
else
  list="$*"
fi

active=0
total=0
for ep in $list; do
  total=$((total+1))
  body=$(curl -fsS --max-time 3 "http://$ep/metrics" 2>/dev/null || true)
  echo "$body" | grep -q '^dvt_up 1' && active=$((active+1))
done

ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"timestamp":"%s","total_endpoints":%d,"active_nodes":%d}\n' "$ts" "$total" "$active"

