# Dependency Whitelist (TSS & P2P)

Scope
- Enforce that any non-std Go modules added to go.mod are explicitly approved and pinned.
- Default runtime does not depend on external cryptography/P2P modules unless enabled via build tags. `blst` is allowed only behind `-tags blst`; P2P modules are allowed only behind `-tags p2p`.

Allowed modules
- github.com/supranational/blst (Go bindings), version pinned (e.g., v0.3.x) — only behind tag `blst`
- github.com/libp2p/go-libp2p (core), version pinned — only behind tag `p2p`
- github.com/libp2p/go-libp2p-pubsub, version pinned — only behind tag `p2p`
- github.com/multiformats/go-multiaddr, version pinned — only behind tag `p2p`

Policy
- Single choke-point import: only `internal/tss/core/bls381` may import the third‑party blst module; only `internal/p2p/*` under `//go:build p2p` may import libp2p/multiaddr.
- Version pinning: exact version in go.mod (for tag builds) or declared in the optional p2p-tag workflow; upgrades via dedicated PR with release notes.
- Security: `govulncheck` and Snyk must be green; license compliance required.
- Build tags: default build MUST NOT import these packages at runtime; only `-tags blst` or `-tags p2p` enables them.

Verification
- CI `dep-whitelist` job runs `./scripts/dep-whitelist.sh` to validate go.mod against this whitelist.
- CI `govulncheck` and Snyk jobs must pass.
- Optional `.github/workflows/p2p-tag.yml` builds with `-tags p2p` using pinned module versions. This workflow is informational and not a required gate.
