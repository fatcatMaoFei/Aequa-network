package private_v1

import "testing"

func TestPool_AddGet(t *testing.T) {
	p := New()
	tx := &PrivateTx{From: "A", Nonce: 0, Ciphertext: []byte{1, 2}, EphemeralKey: []byte{3}, TargetHeight: 10}
	if err := p.Add(tx); err != nil {
		t.Fatalf("add: %v", err)
	}
	if p.Len() != 1 {
		t.Fatalf("len want 1 got %d", p.Len())
	}
	got := p.Get(1, 0)
	if len(got) != 1 {
		t.Fatalf("get len want 1 got %d", len(got))
	}
	if got[0].Type() != "private_v1" {
		t.Fatalf("unexpected type %s", got[0].Type())
	}
}

func TestPool_RejectInvalid(t *testing.T) {
	p := New()
	if err := p.Add(&PrivateTx{From: "", Ciphertext: []byte{1}}); err == nil {
		t.Fatalf("expected error on invalid tx")
	}
}
