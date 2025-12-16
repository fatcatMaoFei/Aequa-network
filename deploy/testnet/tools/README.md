# Testnet KPI Tools (Local Only)

This folder contains helper scripts for KPI collection in the minimal testnet.

active nodes

- Script: `kpi_active.sh`
- Input: file with monitoring endpoints (one `host:port` per line) or arguments
- Output: single JSON line with `timestamp,total_endpoints,active_nodes`

DFBA + BEAST longrun (local soak)

- Script: `dfba_beast_longrun.sh`
- Purpose: run a long-lived 4-node cluster with DFBA/builder/BEAST enabled and BEAST attacker
  traffic, for manual soak testing.
- Usage (from repo root, local only):
  - `RUNTIME_MIN=240 bash deploy/testnet/tools/dfba_beast_longrun.sh`
- Notes:
  - Flags must be configured in `docker-compose.yml` / image build to enable builder (`AEQUA_ENABLE_BUILDER`,
    `--enable-builder`, `--builder.use-dfba`) and BEAST (`--enable-beast`, `p2p` tag, etc.).
  - This helper only orchestrates runtime; it does not change metric/log dimensions and is not wired into CI.

Examples

```
./kpi_active.sh endpoints.sample
./kpi_active.sh 127.0.0.1:4620 127.0.0.1:4621
```

Notes

- Active node is defined as a monitoring endpoint that returns `dvt_up 1`.
- This does not modify code paths or metrics/logging dimensions.
- Keep these tools local-only if your policy forbids committing ops scripts.
