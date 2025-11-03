Node Operator Onboarding (Public Testnet)

Goal

- Join the Aequa minimal DVT testnet (no BEAST鈥慚EV / DFBA). Features are off by default.

Steps

1) Clone repository and build local image:
   git clone https://github.com/zmlAEQ/Aequa-network.git
   cd Aequa-network
   docker build -t aequa-local:latest .

2) Start a node (choose one mapping):
   docker run -d --name aequa-node-operator \
     -p 4600:4600 -p 4620:4620 \
     aequa-local:latest --validator-api 0.0.0.0:4600 --monitoring 0.0.0.0:4620

3) Health & metrics:
   curl http://127.0.0.1:4600/health
   curl http://127.0.0.1:4620/metrics

4) Observability:
   - Import deploy/testnet/grafana/dashboard.json into Grafana
   - Keep labels unchanged; route/code/result/latency_ms/trace_id present in logs

5) Maintenance:
   - Rolling upgrade: stop container then run with the latest image
   - Rollback: run previous image tag

Notes

- In this MVP baseline, P2P manager is minimal and bootnodes.txt is a placeholder.
- Do not enable TSS/BEAST/DFBA yet; these features will ship later behind flags.
- Follow Runbook for operations and alerts baseline.

