Aequa Testnet (Minimal 4-node)

Local-only helper for spinning up a minimal 4-node cluster. By default BEAST/DFBA
remain off; metrics/logs dimensions stay unchanged when you enable them via flags.

Quick start

1) Build the image at repo root:
   docker build -t aequa-local:latest .

2) Start 4 nodes (this folder):
   docker compose -f deploy/testnet/docker-compose.yml up -d

3) Health and metrics:
   curl http://127.0.0.1:4600/health  # node-0
   curl http://127.0.0.1:4620/metrics # node-0

4) Stop and clean:
   docker compose -f deploy/testnet/docker-compose.yml down -v

Notes

- This setup does not enable TSS/BEAST/DFBA by default. It is meant for public testnet
  onboarding and basic observability only. You can opt into DFBA/BEAST experiments with
  extra flags described below without changing existing metric/log dimensions.
- For chaos/adversary tests, prefer the root-level docker-compose.yml.
- Keep this file local-only; do not modify metrics/logging dimensions.

Experimental: enable BEAST P2P path

- Build image with the p2p tag and start a cluster as usual.
- To enable BEAST private tx topic on a node, start `dvt-node` with both:
  - `--enable-beast` (enables private_v1 path locally, still behind API/mempool flags)
  - `--p2p.enable` and a `p2p`-tagged build, plus `--p2p.listen` / `--p2p.bootnodes` as needed.
- When both flags are set, the P2P transport will additionally join the private tx topic
  and gossip `private_v1` payloads over `aequa/tx/private/v1`. Existing metrics/log labels
  remain unchanged; only new metric families are added if needed in future PRs.

Experimental: BEAST threshold mode (DKG + batched decrypt shares)

- Build with tags: `p2p,blst` (example: `docker build --build-arg BUILD_TAGS=p2p,blst -t aequa-p2p-blst:latest .`).
- Start each node with:
  - `--enable-beast`
  - `--p2p.enable` (+ `--p2p.listen` / `--p2p.bootnodes`)
  - `--beast.dkg.conf <path>` (per-node JSON config; contains committee pubkeys and node-specific privkeys)
  - optionally `--enable-builder --builder.use-dfba` to route decrypted flows through DFBA.
- Behaviour (full BEAST/t-of-n batched decrypt):
  - DKG derives a t-of-n BLS381 master key and per-node shares.
  - Private `private_v1` txs are encrypted with BTE+PPRF (via `cmd/beast-encrypt` or JS helper),
    carrying `ciphertext`, `ephemeral_key=C1||C2`, `target_height`, `batch_index`, and `punctured_key`.
  - At `height >= target_height`, each node computes a partial decrypt share `C1^{s_i}` and gossips
    it over `aequa/beast/share/v1`; when >=threshold shares are collected, nodes recover `g^k` and
    the PPRF output, then map back into plaintext/auction flows.
- Observability:
  - DKG: `beast_dkg_total{result}` (e.g. `ok`, `epoch_bump`, `dealer_disqualified`)
  - Decrypt path: `beast_decrypt_total{result}` (e.g. `ok`, `not_ready`, `early`, `cipher_error`)
  - Private pool: `private_pool_in_total{result}`, `private_pool_size`
- Safety: share precompute on private tx ingest is disabled by default; do not enable early share
  publishing on public testnets.

Experimental: deterministic builder + DFBA

- Default off. To route selection through the deterministic builder, start nodes with:
  - environment: `AEQUA_ENABLE_BUILDER=1`
  - flags: `--enable-builder` and optional `--builder.*` tuning flags
- To enable the DFBA path on top of the builder, add:
  - `--builder.use-dfba`
- When enabled, additional metrics become visible in Grafana:
  - DFBA latency/volume: `dfba_solve_ms_*`, `dfba_solve_total{result}`
  - Flow accounting: `dfba_accepted_total{flow}`, `dfba_rejected_total{flow,reason}`
  - Clearing price: `dfba_clearing_price`
  - Block value summaries: `block_value_bids_*`, `block_value_fees_*`
- Keep these flags disabled on public testnets unless you explicitly opt into DFBA experiments.

Optional: KeyShare encryption

- Default off. To enable at-rest encryption for KeyShare storage, set environment variables on the node container:
  - `AEQUA_TSS_KEYSTORE_ENCRYPT=1`
  - Provide a 32-byte key with either `AEQUA_TSS_KEYSTORE_KEY` (hex) or `AEQUA_TSS_KEYSTORE_KEY_FILE` (path to raw bytes).
  - Optional best-effort memory wipe: `AEQUA_TSS_ZEROIZE=1`.
- Risks: losing the key makes current KeyShare unreadable; `.bak` may also be encrypted depending on timing.

Dashboards & Alerts

- Import `deploy/testnet/grafana/dashboard.json` into Grafana (Prometheus datasource uid: Prometheus)
- Optionally import `deploy/testnet/grafana/p2p_panels.json` for P2P visibility (requires p2p build + flags)
- See `deploy/testnet/grafana/alerts.md` for alert suggestions (do not alter label sets)
- For DFBA/BEAST experiments, the main dashboard already exposes DFBA latency/result panels,
  block value averages, and BEAST/private pool metrics (`beast_decrypt_total`, `private_pool_in_total`, `private_pool_size`).
