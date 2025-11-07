Aequa DVT Sequencer

Overview
- A production‑minded, modular Distributed Validator (DVT) sequencer. The core focuses on API/P2P/QBFT/StateDB with optional TSS (behind‑flag), a pluggable mempool, and a deterministic payload builder. Observability and CI/security gates are first‑class and stable (zero label drift).
- Not included yet (under active development): BEAST/MEV and DFBA (batch auction). These will be integrated later as behind‑flag modules without breaking existing metrics/logging dimensions.

Status (Core)
- API: `/v1/duty` with strict validation; unified JSON logs and Prometheus summaries.
- P2P: config‑driven gates (AllowList → Rate → Score) and resource limits; DKG/cluster‑lock precheck (fail‑fast).
- Consensus: QBFT verifier (strict + anti‑replay) and state with preprepare/prepare/commit + dedup; StateDB atomic persist + pessimistic recovery; WAL of vote‑intents.
- Payload/Builder: pluggable mempool container + `plaintext_v1` plugin (strict Nonce, fee‑descending), deterministic selection policy (behind‑flag wiring).
- Observability: stable fields (trace_id/route/code/result/latency_ms) and metric families (`api_requests_total`, `service_op_ms`, `consensus_*`, `qbft_*`, `p2p_*`, `state_*`, `builder_selected_total`, `block_sign_total`).

Quick Start (local)
```bash
# Build
go build -o bin/dvt-node ./cmd/dvt-node

# Run
./bin/dvt-node --validator-api 127.0.0.1:4600 --monitoring 127.0.0.1:4620

# Health
curl http://127.0.0.1:4600/health

# Metrics
curl http://127.0.0.1:4620/metrics
```

Docker (optional)
```bash
docker build -t aequa-local:latest .
docker compose -f deploy/testnet/docker-compose.yml up -d
```

Internal Testnet (multi‑host)
Goal: run Aequa DVT Sequencer across multiple real machines (and optionally add a few containerized nodes to enlarge the cluster). This guide does not involve BEAST/DFBA.

Prerequisites
- 3–7 Linux/Windows machines with reachable network
- Go 1.24
- Open ports per node: API 4600 (TCP) and Metrics 4620 (TCP)
- Optional: one monitoring host with Prometheus/Grafana

Way A — native binary (recommended)
1) Build on each machine:
```bash
git clone https://github.com/zmlAEQ/Aequa-network
cd Aequa-network
go build -o dvt-node ./cmd/dvt-node
```
2) Start one node per machine:
```bash
./dvt-node \
  --validator-api 0.0.0.0:4600 \
  --monitoring   0.0.0.0:4620
```
Optional flags:
- Enable TSS session service (behind‑flag; default off): `--enable-tss`

3) Check:
- Health: `curl http://<host>:4600/health`
- Metrics: `curl http://<host>:4620/metrics`

4) (Optional) systemd service example:
```ini
[Unit]
Description=Aequa DVT Node
After=network.target

[Service]
ExecStart=/opt/aequa/dvt-node --validator-api 0.0.0.0:4600 --monitoring 0.0.0.0:4620
Restart=always
RestartSec=5
WorkingDirectory=/opt/aequa

[Install]
WantedBy=multi-user.target
```

Way B — add extra nodes via Docker (hybrid)
```bash
docker build -t aequa-local:latest .
docker compose -f deploy/testnet/docker-compose.yml up -d
```
This compose spins up 4 additional nodes mapping 4600..4603 (API) and 4620..4623 (metrics).

Observability & Dashboards
- Import `deploy/testnet/grafana/dashboard.json` into Grafana (Prometheus datasource uid: Prometheus).
- Suggested alerts: timeouts >1% (10m); TSS service avg >200ms (10m).

Long‑run & Chaos
- Baseline long‑run: `.github/workflows/m4-longrun.yml` (API/QBFT fuzz, P2P+DKG/StateDB stress, and builder/payload loops).
- Chaos (netem+churn): `.github/workflows/m4-longrun-chaos.yml`.

CI / Security Gates
- Go 1.24; golangci‑lint; unit tests with coverage gate; `govulncheck`; Snyk CLI; UTF‑8 no‑BOM check (blocking); QBFT tests.

License
Business Source License 1.1 (BSL 1.1). See LICENSE for details.

