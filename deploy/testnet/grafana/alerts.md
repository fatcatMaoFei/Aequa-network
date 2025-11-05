Grafana/Prometheus Alert Suggestions (Local Only)

This file lists suggested alert rules aligned with our stable metrics/logging dimensions. Keep features off by default; adjust thresholds in ops.

Suggested alerts

- API 5xx rate
  - expr: sum(rate(api_requests_total{code=~"5.."}[5m])) / sum(rate(api_requests_total[5m])) > 0.01
  - for: 10m
  - labels: severity=page
  - annotations: summary="API 5xx >1%"

- API latency average spike
  - expr: (sum(rate(api_latency_ms_sum[5m])) / sum(rate(api_latency_ms_count[5m]))) > 200
  - for: 10m
  - labels: severity=warn
  - annotations: summary="API avg latency >200ms"

- Consensus processing latency spike
  - expr: (sum(rate(consensus_proc_ms_sum[5m])) / sum(rate(consensus_proc_ms_count[5m]))) > 300
  - for: 10m
  - labels: severity=warn
  - annotations: summary="consensus avg latency >300ms"

- State persist errors
  - expr: increase(state_persist_errors_total[10m]) > 0
  - for: 0m
  - labels: severity=page
  - annotations: summary="state persist errors detected"

- P2P gate anomalies
  - expr: sum(rate(p2p_conn_attempts_total{result=~"limited|dkg_denied"}[5m])) > 1
  - for: 15m
  - labels: severity=warn
  - annotations: summary="p2p gate denials elevated"

- TSS session timeouts (if enabled in future)
  - expr: sum(rate(tss_sessions_total{result="timeout"}[10m])) > 0
  - for: 0m
  - labels: severity=warn
  - annotations: summary="TSS timeouts observed"

Note
- Keep label sets unchanged; use only existing families and labels.
- These rules are suggestions for ops; do not commit to remote if policy forbids.


TSS gray rollout watch (add if enabled)
- expr: sum(rate(tss_rate_limited_total[5m])) > 0
  for: 10m
  labels: severity=info
  annotations: summary="TSS limiter activity observed"
- expr: tss_sessions_open > 0
  for: 0m
  labels: severity=info
  annotations: summary="TSS sessions open"