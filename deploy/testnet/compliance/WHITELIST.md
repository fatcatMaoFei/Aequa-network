# Dependency Whitelist (TSS)

Scope
- Enforce that any non-std Go modules added to go.mod are explicitly approved and pinned.
- Default build uses no external deps. blst is allowed only behind `-tags blst` and must be pinned.

Allowed modules
- github.com/supranational/blst (Go bindings), version pinned (e.g., v0.3.x) — only behind tag `blst`
- (add here if future audited modules are required)

Policy
- Single choke-point import: only `internal/tss/core/bls381` may import the third‑party blst module.
- Version pinning: exact version in go.mod; upgrades via dedicated PR with release notes.
- Security: `govulncheck` and Snyk must be green; license compliance required.
- Build tags: default build MUST NOT require blst; only `-tags blst` enables it.

Verification
- CI `dep-whitelist` job runs `./scripts/dep-whitelist.sh` to validate go.mod against this whitelist.
- CI `govulncheck` and Snyk jobs must pass.