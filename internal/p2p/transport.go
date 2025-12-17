package p2p

import (
	"context"

	qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
	"github.com/zmlAEQ/Aequa-network/internal/p2p/wire"
	"github.com/zmlAEQ/Aequa-network/internal/payload"
)

// Transport defines a minimal P2P transport abstraction used by the node.
// Phase 1 provides the interface only (no concrete network dependency here).
// Implementations (e.g., libp2p+gossipsub) live behind feature flags.
type Transport interface {
	// Start brings up the network stack and subscriptions.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the network stack and subscriptions.
	Stop(ctx context.Context) error

	// BroadcastQBFT publishes a QBFT message to the consensus topic.
	BroadcastQBFT(ctx context.Context, msg qbft.Message) error
	// BroadcastTx publishes a payload/transaction to the tx topic.
	BroadcastTx(ctx context.Context, tx payload.Payload) error

	// OnQBFT registers a handler invoked on each inbound QBFT message.
	OnQBFT(fn func(qbft.Message))
	// OnTx registers a handler invoked on each inbound transaction/payload.
	OnTx(fn func(payload.Payload))
}

// BeastShareTransport is an optional extension implemented by transports that
// support BEAST decryption share gossip.
type BeastShareTransport interface {
	// BroadcastBeastShare publishes a BEAST share message to the share topic.
	BroadcastBeastShare(ctx context.Context, msg wire.BeastShare) error
	// OnBeastShare registers a handler invoked on each inbound BEAST share.
	OnBeastShare(fn func(wire.BeastShare))
}

// NoopTransport is a stub implementation used when P2P is disabled.
// It satisfies the interface without performing any network I/O.
type NoopTransport struct {
	onQBFT       func(qbft.Message)
	onTx         func(payload.Payload)
	onBeastShare func(wire.BeastShare)
}

func (n *NoopTransport) Start(_ context.Context) error { return nil }
func (n *NoopTransport) Stop(_ context.Context) error  { return nil }

func (n *NoopTransport) BroadcastQBFT(_ context.Context, _ qbft.Message) error          { return nil }
func (n *NoopTransport) BroadcastTx(_ context.Context, _ payload.Payload) error         { return nil }
func (n *NoopTransport) BroadcastBeastShare(_ context.Context, _ wire.BeastShare) error { return nil }

func (n *NoopTransport) OnQBFT(fn func(qbft.Message))          { n.onQBFT = fn }
func (n *NoopTransport) OnTx(fn func(payload.Payload))         { n.onTx = fn }
func (n *NoopTransport) OnBeastShare(fn func(wire.BeastShare)) { n.onBeastShare = fn }
