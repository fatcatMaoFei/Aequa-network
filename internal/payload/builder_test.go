package payload

import (
    "testing"
    pt "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

func TestPrepareProposal_Plaintext_SortsByPolicyAndFee(t *testing.T) {
    pool := pt.New()
    c := NewContainer(map[string]TypedMempool{"plaintext_v1": pool})
    _ = c.Add(&pt.PlaintextTx{From:"A", Nonce:0, Gas:1, Fee:5, Sig: make([]byte,32)})
    _ = c.Add(&pt.PlaintextTx{From:"B", Nonce:0, Gas:1, Fee:10, Sig: make([]byte,32)})
    _ = c.Add(&pt.PlaintextTx{From:"A", Nonce:1, Gas:1, Fee:7, Sig: make([]byte,32)})
    pol := BuilderPolicy{Order: []string{"plaintext_v1"}, MaxN: 3}
    blk := PrepareProposal(c, BlockHeader{Height:1, Round:1}, pol)
    if len(blk.Items) != 3 { t.Fatalf("want 3 items, got %d", len(blk.Items)) }
    // fees should be non-increasing: 10 >= 7 >= 5 (pending/promoted)
    if !(blk.Items[0].SortKey() >= blk.Items[1].SortKey() && blk.Items[1].SortKey() >= blk.Items[2].SortKey()) {
        t.Fatalf("not sorted by fee desc")
    }
    if err := ProcessProposal(blk, pol); err != nil { t.Fatalf("process: %v", err) }
}

func TestProcessProposal_RejectsWrongOrder(t *testing.T) {
    pol := BuilderPolicy{Order: []string{"plaintext_v1"}, MaxN: 10}
    // construct a block with wrong fee order (increasing)
    items := []Payload{
        &pt.PlaintextTx{From:"A", Nonce:0, Gas:1, Fee:1, Sig: make([]byte,32)},
        &pt.PlaintextTx{From:"B", Nonce:0, Gas:1, Fee:5, Sig: make([]byte,32)},
    }
    blk := StandardBlock{Header: BlockHeader{Height:1, Round:1}, Items: items}
    if err := ProcessProposal(blk, pol); err == nil {
        t.Fatalf("want error on bad order")
    }
}

