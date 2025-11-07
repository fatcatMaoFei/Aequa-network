package payload_test

import (
    "testing"
    payload "github.com/zmlAEQ/Aequa-network/internal/payload"
    pt "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

// dummy implements Payload for routing tests
type dummy struct{ t string }
func (d dummy) Type() string { return d.t }
func (d dummy) Hash() []byte { return nil }
func (d dummy) Validate() error { return nil }
func (d dummy) SortKey() uint64 { return 0 }

func TestContainer_RoutesToTypedPool(t *testing.T) {
    pool := pt.New()
    c := payload.NewContainer(map[string]payload.TypedMempool{"plaintext_v1": pool})
    // add via a proper typed payload instance
    if err := c.Add(&pt.PlaintextTx{From:"X", Nonce:0, Gas:1, Fee:1, Sig: make([]byte,32)}); err != nil {
        t.Fatalf("add: %v", err)
    }
    if c.Len() != 1 { t.Fatalf("want total len 1, got %d", c.Len()) }
    // unknown type is ignored
    _ = c.Add(dummy{t:"unknown"})
}
