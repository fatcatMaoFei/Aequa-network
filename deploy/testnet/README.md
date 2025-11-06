Aequa Testnet (Minimal 4-node)

Local-only helper for spinning up a minimal 4-node cluster without BEAST/DFBA.
Features remain off by default; metrics/logs dimensions stay unchanged.

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

- This setup does not enable TSS/BEAST/DFBA. It is meant for public testnet
  onboarding and basic observability only.
- For chaos/adversary tests, prefer the root-level docker-compose.yml.
- Keep this file local-only; do not modify metrics/logging dimensions.

Optional: KeyShare encryption

- Default off. To enable at-rest encryption for KeyShare storage, set environment variables on the node container:
  - `AEQUA_TSS_KEYSTORE_ENCRYPT=1`
  - Provide a 32-byte key with either `AEQUA_TSS_KEYSTORE_KEY` (hex) or `AEQUA_TSS_KEYSTORE_KEY_FILE` (path to raw bytes).
  - Optional best-effort memory wipe: `AEQUA_TSS_ZEROIZE=1`.
- Risks: losing the key makes current KeyShare unreadable; `.bak` may also be encrypted depending on timing.
