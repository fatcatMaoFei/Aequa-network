package auction_bid_v1

import (
	"testing"
)

func TestPool_AddAndGetOrdersByBid(t *testing.T) {
	p := New()
	// add three bids with different bid values
	_ = p.Add(&AuctionBidTx{From: "A", Nonce: 0, Gas: 1, Bid: 10, FeeRecipient: "r", Sig: make([]byte, 32)})
	_ = p.Add(&AuctionBidTx{From: "B", Nonce: 0, Gas: 1, Bid: 20, FeeRecipient: "r", Sig: make([]byte, 32)})
	_ = p.Add(&AuctionBidTx{From: "C", Nonce: 0, Gas: 1, Bid: 15, FeeRecipient: "r", Sig: make([]byte, 32)})

	got := p.Get(3, 0)
	if len(got) != 3 {
		t.Fatalf("expected 3 tx, got %d", len(got))
	}
	if got[0].(*AuctionBidTx).Bid != 20 || got[1].(*AuctionBidTx).Bid != 15 || got[2].(*AuctionBidTx).Bid != 10 {
		t.Fatalf("unexpected order: %+v %+v %+v", got[0], got[1], got[2])
	}
}

func TestPool_FuturePromotion(t *testing.T) {
	p := New()
	if err := p.Add(&AuctionBidTx{From: "A", Nonce: 1, Gas: 1, Bid: 5, FeeRecipient: "r", Sig: make([]byte, 32)}); err != nil {
		t.Fatalf("add future: %v", err)
	}
	if err := p.Add(&AuctionBidTx{From: "A", Nonce: 0, Gas: 1, Bid: 7, FeeRecipient: "r", Sig: make([]byte, 32)}); err != nil {
		t.Fatalf("add ready: %v", err)
	}
	got := p.Get(2, 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 tx, got %d", len(got))
	}
	if got[0].(*AuctionBidTx).Nonce != 0 || got[1].(*AuctionBidTx).Nonce != 1 {
		t.Fatalf("unexpected nonce order: %d %d", got[0].(*AuctionBidTx).Nonce, got[1].(*AuctionBidTx).Nonce)
	}
}

func TestPool_RejectInvalid(t *testing.T) {
	p := New()
	if err := p.Add(&AuctionBidTx{From: "", Nonce: 0, Gas: 1, Bid: 1, FeeRecipient: "r", Sig: make([]byte, 32)}); err == nil {
		t.Fatalf("expected error for empty from")
	}
	if err := p.Add(&AuctionBidTx{From: "A", Nonce: 0, Gas: 0, Bid: 1, FeeRecipient: "r", Sig: make([]byte, 32)}); err == nil {
		t.Fatalf("expected error for zero gas")
	}
	if err := p.Add(&AuctionBidTx{From: "A", Nonce: 0, Gas: 1, Bid: 0, FeeRecipient: "r", Sig: make([]byte, 32)}); err == nil {
		t.Fatalf("expected error for zero bid")
	}
	if err := p.Add(&AuctionBidTx{From: "A", Nonce: 0, Gas: 1, Bid: 1, FeeRecipient: "", Sig: make([]byte, 32)}); err == nil {
		t.Fatalf("expected error for empty fee recipient")
	}
	if err := p.Add(&AuctionBidTx{From: "A", Nonce: 0, Gas: 1, Bid: 1, FeeRecipient: "r", Sig: make([]byte, 8)}); err == nil {
		t.Fatalf("expected error for short sig")
	}
}
