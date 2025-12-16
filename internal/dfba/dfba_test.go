package dfba

import "testing"

func TestSolveDeterministic_DualFlowMatchesBidsAndUsers(t *testing.T) {
	items := []Item{
		{Type: "auction_bid_v1", Key: 10, Hash: []byte{1}},
		{Type: "auction_bid_v1", Key: 5, Hash: []byte{2}},
		{Type: "plaintext_v1", Key: 7, Hash: []byte{3}},
		{Type: "plaintext_v1", Key: 3, Hash: []byte{4}},
	}
	pol := Policy{
		Order:  []string{"auction_bid_v1", "plaintext_v1"},
		MaxN:   4,
		Window: 2,
	}
	out, err := SolveDeterministic(SolverInput{Items: items, Policy: pol})
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if len(out.Selected) != 4 {
		t.Fatalf("expected 4 items (2 pairs), got %d", len(out.Selected))
	}
	// first two should be bids (type order) and highest keys kept
	if out.Selected[0].Type != "auction_bid_v1" || out.Selected[1].Type != "auction_bid_v1" {
		t.Fatalf("expected bids first, got %s %s", out.Selected[0].Type, out.Selected[1].Type)
	}
	if out.Selected[0].Key != 10 || out.Selected[1].Key != 5 {
		t.Fatalf("unexpected bid keys: %d %d", out.Selected[0].Key, out.Selected[1].Key)
	}
	if out.Selected[2].Type != "plaintext_v1" || out.Selected[3].Type != "plaintext_v1" {
		t.Fatalf("expected users after bids, got %s %s", out.Selected[2].Type, out.Selected[3].Type)
	}
	if out.Selected[2].Key != 7 || out.Selected[3].Key != 3 {
		t.Fatalf("unexpected user keys: %d %d", out.Selected[2].Key, out.Selected[3].Key)
	}
}

