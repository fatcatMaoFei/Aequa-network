package bus

import (
	"context"
)

type Kind string

const (
    KindDuty Kind = "duty"
    // KindConsensus represents an inbound consensus (QBFT) message delivered
    // from the network transport into the internal bus.
    KindConsensus Kind = "consensus"
    // KindTx is reserved for future use (transaction gossip to mempool).
    KindTx Kind = "tx"
)

type Event struct {
	Kind    Kind
	Height  uint64
	Round   uint64
	Body    any
	TraceID string
}

type Subscriber chan Event

type Bus struct {
	pub chan Event
}

func New(size int) *Bus {
	if size <= 0 { size = 128 }
	return &Bus{pub: make(chan Event, size)}
}

func (b *Bus) Publish(_ context.Context, ev Event) {
	select { case b.pub <- ev: default: /* drop on backpressure */ }
}

func (b *Bus) Subscribe() Subscriber { return b.pub }
