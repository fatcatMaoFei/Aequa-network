# Dependency Whitelist (TSS & P2P)

Scope
- Enforce that any non-std Go modules added to go.mod are explicitly approved and pinned.
- Default runtime does not depend on external cryptography/P2P modules unless enabled via build tags. `blst` is allowed only behind `-tags blst`; P2P modules are allowed only behind `-tags p2p`.

Allowed modules
- github.com/supranational/blst (Go bindings), version pinned (e.g., v0.3.x) 鈥?only behind tag `blst`
- (add here if future audited modules are required)

Policy
- Single choke-point import: only `internal/tss/core/bls381` may import the third鈥憄arty blst module.
- Version pinning: exact version in go.mod; upgrades via dedicated PR with release notes.
- Security: `govulncheck` and Snyk must be green; license compliance required.
- Build tags: default build MUST NOT require blst; only `-tags blst` enables it.

Verification
- CI `dep-whitelist` job runs `./scripts/dep-whitelist.sh` to validate go.mod against this whitelist.
- CI `govulncheck` and Snyk jobs must pass.
