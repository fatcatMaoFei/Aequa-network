package p2p

// NetConfig carries runtime options for the P2P transport.
// It is intentionally small for Phase 1; more options can be added later.
type NetConfig struct {
    Enable    bool
    Listen    []string // multiaddrs to listen on; empty => libp2p default
    Bootnodes []string // multiaddrs to dial on start
    NAT       bool     // enable NAT port mapping if available
}

