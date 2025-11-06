Aequa DVT Testnet Runbook (Local Ops Guide)

Scope

- Minimal public testnet without BEAST鈥慚EV / DFBA. Features stay off by default.
- Operate 4鈥? nodes using the provided Docker image and compose templates.
- Keep metrics/logging dimensions unchanged. Use dashboards under deploy/testnet/grafana.

Prerequisites

- Docker 24+ and docker compose plugin
- Host ports available: 4600..4623 (for 4 nodes example)

Build & Start

1) Build image at repo root (tags optional):
   docker build -t aequa-local:latest .

2) Start minimal 4 nodes (no adversary):
   docker compose -f deploy/testnet/docker-compose.yml up -d

3) Health & metrics:
   curl http://127.0.0.1:4600/health   # -> ok
   curl http://127.0.0.1:4620/metrics  # Prom exposition

Stop & Clean

docker compose -f deploy/testnet/docker-compose.yml down -v

Observability (baseline)

- Logs (stdout): JSON lines with fields: trace_id, route, code, result, latency_ms, err?
- Metrics (Prometheus text): stable families
  - api_requests_total{route,code}, api_latency_ms_sum/_count{route}
  - service_op_ms_sum/_count{service,op}
  - consensus_events_total{kind}, consensus_proc_ms_sum/_count{kind}
  - qbft_msg_verified_total{result|type}, qbft_state_transitions_total{type}
  - p2p_conn_attempts_total{result}, p2p_conns_open, p2p_conn_open_total/close_total
  - state_persist_ms_sum/_count, state_recovery_total{result}
  - tss_rate_limited_total{kind}, tss_sessions_open, tss_sessions_total{result}

Dashboards & Alerts

- Import deploy/testnet/grafana/dashboard.json into Grafana (Prometheus datasource uid: Prometheus)
- See deploy/testnet/grafana/alerts.md for alert suggestions (do not alter label sets)

Rolling Upgrade (no BEAST/DFBA, features off)

1) Build new image with version label:
   docker build -t aequa-local:latest .

2) Upgrade node by node (serial):
   docker compose -f deploy/testnet/docker-compose.yml stop node-0 && \
   docker compose -f deploy/testnet/docker-compose.yml up -d node-0

3) Check health & metrics before moving to the next node.

Rollback

- Rebuild previous known-good image or retag, then repeat the rolling procedure.
- If cluster becomes unstable: stop all, bring up previous image for all nodes.

Troubleshooting Checklist

- Ports busy: ensure 4600/4620(+N) are free, or adjust port mappings in compose.
- Metrics missing: confirm monitoring port mapping; verify dashboard datasource uid.
- Elevated 5xx / latency spikes: check host resource pressure; restart node and watch service_op_ms.
- P2P denials/limits: confirm Config gate presets; in this baseline P2P manager is minimal, gates may be no-op.

Security Notes

- Features (TSS/BEAST/DFBA) remain disabled. Do not expose testnet to untrusted networks.
- No external dependencies introduced; keep image up to date and rebuild routinely.

Change Management

- Never change metrics/log labels in this baseline. Only add new metric families when needed.
- Use small PRs and feature flags (default off). Revert by disabling features or rolling back images.


Gray Rollout (TSS behind flag)

- Default off. To enable on a single node (canary):
  docker compose -f deploy/testnet/docker-compose.yml stop node-0 && \
  docker run -d --name aequa-testnet-canary \
    -p 4600:4600 -p 4620:4620 aequa-local:latest \
    --validator-api 0.0.0.0:4600 --monitoring 0.0.0.0:4620 --enable-tss
- Observe dashboards: tss_sessions_open, tss_rate_limited_total, service_op_ms{service="tss"}
- If stable, roll TSS to remaining nodes (one by one) using rolling_upgrade.sh and adding --enable-tss to command.
- To disable, restart container without the flag (or use docker-compose.yml baseline).
SLO & Benchmarks (TSS)

- Benchmarks (manual):
  ./deploy/testnet/tools/tss_bench.sh  # runs -tags blst benches under internal/tss/core/bls381
  Record p50/p95 from output (ns/op) as baseline; attach to release notes.
- Dashboards: ensure panel "TSS service op avg (ms)" is visible after enabling TSS.
- Alerts (suggested): timeouts >1% for 10m; tss service avg >200ms for 10m.