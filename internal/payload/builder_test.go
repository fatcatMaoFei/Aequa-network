package payload_test

import (
	"testing"
	"time"

	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// dummyPayload implements payload.Payload for testing DFBA logic.
type dummyPayload struct {
	t   string
	key uint64
}

func (d *dummyPayload) Type() string    { return d.t }
func (d *dummyPayload) Hash() []byte    { return []byte{byte(d.key)} }
func (d *dummyPayload) Validate() error { return nil }
func (d *dummyPayload) SortKey() uint64 { return d.key }

// dummyPool is a simple pool that returns items in insertion order.
type dummyPool struct{ items []payload.Payload }

func (p *dummyPool) Add(pl payload.Payload) error {
	p.items = append(p.items, pl)
	return nil
}

func (p *dummyPool) Get(n int, _ int) []payload.Payload {
	if n <= 0 || n > len(p.items) {
		n = len(p.items)
	}
	out := p.items[:n]
	return out
}
func (p *dummyPool) Len() int { return len(p.items) }

func TestPrepareProposal_SortsWithinType(t *testing.T) {
	pool := &dummyPool{}
	c := payload.NewContainer(map[string]payload.TypedMempool{
		"plaintext_v1": pool,
	})
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 5})
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 10})
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 7})
	pol := payload.BuilderPolicy{Order: []string{"plaintext_v1"}, MaxN: 3}
	blk := payload.PrepareProposal(c, payload.BlockHeader{Height: 1, Round: 1}, pol)
	if len(blk.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(blk.Items))
	}
	if !(blk.Items[0].SortKey() >= blk.Items[1].SortKey() && blk.Items[1].SortKey() >= blk.Items[2].SortKey()) {
		t.Fatalf("not sorted by key desc")
	}
	if err := payload.ProcessProposal(blk, pol); err != nil {
		t.Fatalf("process: %v", err)
	}
}

func TestPrepareProposal_EnforcesThresholds(t *testing.T) {
	bidPool := &dummyPool{}
	feePool := &dummyPool{}
	c := payload.NewContainer(map[string]payload.TypedMempool{
		"auction_bid_v1": bidPool,
		"plaintext_v1":   feePool,
	})
	_ = c.Add(&dummyPayload{t: "auction_bid_v1", key: 5})
	_ = c.Add(&dummyPayload{t: "auction_bid_v1", key: 15})
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 1})
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 12})
	pol := payload.BuilderPolicy{
		Order:  []string{"auction_bid_v1", "plaintext_v1"},
		MaxN:   4,
		MinBid: 10,
		MinFee: 10,
	}
	blk := payload.PrepareProposal(c, payload.BlockHeader{Height: 1, Round: 1}, pol)
	if len(blk.Items) != 2 {
		t.Fatalf("expected 2 items after thresholds, got %d", len(blk.Items))
	}
	for _, it := range blk.Items {
		if it.Type() == "auction_bid_v1" && it.SortKey() < pol.MinBid {
			t.Fatalf("bid below threshold")
		}
		if it.Type() == "plaintext_v1" && it.SortKey() < pol.MinFee {
			t.Fatalf("fee below threshold")
		}
	}
}

func TestPrepareProposal_WindowPerType(t *testing.T) {
	bidPool := &dummyPool{}
	feePool := &dummyPool{}
	c := payload.NewContainer(map[string]payload.TypedMempool{
		"auction_bid_v1": bidPool,
		"plaintext_v1":   feePool,
	})
	// add more than window
	for i := 0; i < 3; i++ {
		_ = c.Add(&dummyPayload{t: "auction_bid_v1", key: uint64(100 - i)})
	}
	for i := 0; i < 3; i++ {
		_ = c.Add(&dummyPayload{t: "plaintext_v1", key: uint64(50 - i)})
	}
	pol := payload.BuilderPolicy{
		Order:      []string{"auction_bid_v1", "plaintext_v1"},
		MaxN:       6,
		Window:     2,
		BatchTicks: 50, // ms
	}
	blk := payload.PrepareProposal(c, payload.BlockHeader{Height: 1, Round: 1}, pol)
	if len(blk.Items) != 4 {
		t.Fatalf("expected 4 items (2 per type window), got %d", len(blk.Items))
	}
	// first two must be bids (order enforced), last two plaintext
	for i := 0; i < 2; i++ {
		if blk.Items[i].Type() != "auction_bid_v1" {
			t.Fatalf("expected auction first, got %s", blk.Items[i].Type())
		}
	}
	for i := 2; i < 4; i++ {
		if blk.Items[i].Type() != "plaintext_v1" {
			t.Fatalf("expected plaintext after bids, got %s", blk.Items[i].Type())
		}
	}
	if err := payload.ProcessProposal(blk, pol); err != nil {
		t.Fatalf("process: %v", err)
	}
}

func TestPrepareProposal_ArrivalOrderRespected(t *testing.T) {
	pool := &dummyPool{}
	c := payload.NewContainer(map[string]payload.TypedMempool{"plaintext_v1": pool})
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 5})
	time.Sleep(2 * time.Millisecond)
	_ = c.Add(&dummyPayload{t: "plaintext_v1", key: 100}) // higher fee but later
	pol := payload.BuilderPolicy{Order: []string{"plaintext_v1"}, MaxN: 2, BatchTicks: 100}
	blk := payload.PrepareProposal(c, payload.BlockHeader{Height: 1, Round: 1}, pol)
	if len(blk.Items) != 2 {
		t.Fatalf("expected 2 items")
	}
	meta0, _ := c.Arrival(blk.Items[0])
	meta1, _ := c.Arrival(blk.Items[1])
	if meta0.seq >= meta1.seq {
		t.Fatalf("arrival order not respected: seq0=%d seq1=%d", meta0.seq, meta1.seq)
	}
}
