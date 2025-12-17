package consensus

import (
	"context"
	"testing"
	"time"

	pl "github.com/zmlAEQ/Aequa-network/internal/payload"
	pt "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
	"github.com/zmlAEQ/Aequa-network/pkg/bus"
)

func TestService_BuilderRuns_WhenEnabled(t *testing.T) {
	b := bus.New(4)
	s := NewWithSub(b.Subscribe())
	s.SetVerifier(nopVerifier{})
	// enable behind-flag
	s.enableBuilder = true
	// wire container and pool with some txs
	pool := pt.New()
	c := pl.NewContainer(map[string]pl.TypedMempool{"plaintext_v1": pool})
	_ = c.Add(&pt.PlaintextTx{From: "A", Nonce: 0, Gas: 1, Fee: 1, Sig: make([]byte, 32)})
	s.SetPayloadContainer(c)
	s.SetBuilderPolicy(pl.BuilderPolicy{Order: []string{"plaintext_v1"}, MaxN: 1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	// publish a single event -> should trigger builder path without panic
	b.Publish(ctx, bus.Event{Kind: bus.KindDuty, Height: 1, Round: 0})
	time.Sleep(30 * time.Millisecond)
}

func TestService_BuilderUsesDefaultPolicy_WhenNoneProvided(t *testing.T) {
	b := bus.New(4)
	s := NewWithSub(b.Subscribe())
	s.SetVerifier(nopVerifier{})
	// enable builder but do not set policy explicitly
	s.enableBuilder = true
	pool := pt.New()
	c := pl.NewContainer(map[string]pl.TypedMempool{"plaintext_v1": pool})
	_ = c.Add(&pt.PlaintextTx{From: "A", Nonce: 0, Gas: 1, Fee: 2, Sig: make([]byte, 32)})
	s.SetPayloadContainer(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	b.Publish(ctx, bus.Event{Kind: bus.KindDuty, Height: 2, Round: 0})
	time.Sleep(50 * time.Millisecond)
	// builder should have produced a block under default policy
	if blk, ok := s.lastBlock[2][0]; !ok || len(blk.Items) == 0 {
		t.Fatalf("expected default builder policy to select tx, got: %#v", blk)
	}
}
