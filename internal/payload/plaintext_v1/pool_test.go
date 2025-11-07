package plaintext_v1

import (
    "testing"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func tx(from string, nonce, fee uint64) *PlaintextTx {
    return &PlaintextTx{From: from, Nonce: nonce, Gas: 1, Fee: fee, Sig: make([]byte, 32)}
}

func TestPool_Add_PendingAndFuturePromotion(t *testing.T) {
    metrics.Reset()
    p := New()
    // add out-of-order: nonce 1 goes to future
    if err := p.Add(tx("A", 1, 10)); err != nil { t.Fatalf("add future: %v", err) }
    if p.Len() != 0 { t.Fatalf("pending should be 0") }
    // add expected nonce 0 -> pending, then future[1] promoted
    if err := p.Add(tx("A", 0, 5)); err != nil { t.Fatalf("add pending: %v", err) }
    if p.Len() != 2 { t.Fatalf("want 2 pending, got %d", p.Len()) }
}

func TestPool_Add_ReplayAndDup(t *testing.T) {
    metrics.Reset()
    p := New()
    _ = p.Add(tx("B", 0, 1))
    // old nonce rejected
    if err := p.Add(tx("B", 0, 2)); err == nil { t.Fatalf("want old nonce error") }
    // duplicate future rejected
    if err := p.Add(tx("B", 2, 1)); err != nil { t.Fatalf("add future: %v", err) }
    if err := p.Add(tx("B", 2, 3)); err == nil { t.Fatalf("want dup future error") }
}

func TestPool_Get_SortsByFee(t *testing.T) {
    p := New()
    _ = p.Add(tx("A", 0, 5))
    _ = p.Add(tx("B", 0, 10))
    out := p.Get(2, 0)
    if len(out) != 2 { t.Fatalf("want 2") }
    if out[0].(*PlaintextTx).Fee < out[1].(*PlaintextTx).Fee { t.Fatalf("order not by fee desc") }
}

