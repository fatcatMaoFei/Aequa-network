package consensus

import (
    "context"
    "testing"
    "time"
    "github.com/zmlAEQ/Aequa-network/pkg/bus"
    pl "github.com/zmlAEQ/Aequa-network/internal/payload"
    pt "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

// stub signer records calls
type recSigner struct{ called int }
func (r *recSigner) Sign(_ context.Context, _ uint64, _ uint64, _ []byte) ([]byte, error) { r.called++; return []byte("ok"), nil }

func TestService_BuilderRuns_WhenEnabled(t *testing.T) {
    b := bus.New(4)
    s := NewWithSub(b.Subscribe())
    s.SetVerifier(nopVerifier{})
    // enable behind-flag
    s.enableBuilder = true
    // wire container and pool with some txs
    pool := pt.New()
    c := pl.NewContainer(map[string]pl.TypedMempool{"plaintext_v1": pool})
    _ = c.Add(&pt.PlaintextTx{From:"A", Nonce:0, Gas:1, Fee:1, Sig: make([]byte,32)})
    s.SetPayloadContainer(c)
    s.SetBuilderPolicy(pl.BuilderPolicy{Order: []string{"plaintext_v1"}, MaxN: 1})

    ctx, cancel := context.WithCancel(context.Background()); defer cancel()
    if err := s.Start(ctx); err != nil { t.Fatalf("start: %v", err) }
    // publish a single event -> should trigger builder path without panic
    b.Publish(ctx, bus.Event{Kind: bus.KindDuty, Height: 1, Round: 1})
    time.Sleep(30 * time.Millisecond)
}

