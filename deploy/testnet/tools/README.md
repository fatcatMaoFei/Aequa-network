# Testnet KPI Tools (Local Only)

This folder contains helper scripts for KPI collection in the minimal testnet.

active nodes

- Script: `kpi_active.sh`
- Input: file with monitoring endpoints (one `host:port` per line) or arguments
- Output: single JSON line with `timestamp,total_endpoints,active_nodes`

Examples

```
./kpi_active.sh endpoints.sample
./kpi_active.sh 127.0.0.1:4620 127.0.0.1:4621
```

Notes

- Active node is defined as a monitoring endpoint that returns `dvt_up 1`.
- This does not modify code paths or metrics/logging dimensions.
- Keep these tools local-only if your policy forbids committing ops scripts.

