//go:build blst

package dkg

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zmlAEQ/Aequa-network/internal/p2p/wire"

	blst "github.com/supranational/blst/bindings/go"
)

type memDKGBus struct {
	mu   sync.Mutex
	subs []func(wire.TSSDKG)
}

func (b *memDKGBus) subscribe(fn func(wire.TSSDKG)) {
	b.mu.Lock()
	b.subs = append(b.subs, fn)
	b.mu.Unlock()
}

func (b *memDKGBus) broadcast(m wire.TSSDKG) {
	b.mu.Lock()
	subs := append([]func(wire.TSSDKG){}, b.subs...)
	b.mu.Unlock()
	for _, fn := range subs {
		fn := fn
		go fn(m)
	}
}

type memDKGTransport struct {
	bus *memDKGBus
}

func (t *memDKGTransport) BroadcastTSSDKG(_ context.Context, msg wire.TSSDKG) error {
	t.bus.broadcast(msg)
	return nil
}

func (t *memDKGTransport) OnTSSDKG(fn func(wire.TSSDKG)) {
	t.bus.subscribe(fn)
}

func TestBeastDKGRunner_ClosedLoop_OK(t *testing.T) {
	const (
		n = 4
		k = 3
	)

	type keys struct {
		sigPriv []byte
		sigPub  []byte
		encPriv []byte
		encPub  []byte
	}
	nodeKeys := make(map[int]keys, n)
	committee := make([]BeastDKGMember, 0, n)
	for i := 1; i <= n; i++ {
		sigPub, sigPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("ed25519: %v", err)
		}
		encPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("x25519: %v", err)
		}
		encPub := encPriv.PublicKey()

		nodeKeys[i] = keys{
			sigPriv: append([]byte(nil), sigPriv...),
			sigPub:  append([]byte(nil), sigPub...),
			encPriv: append([]byte(nil), encPriv.Bytes()...),
			encPub:  append([]byte(nil), encPub.Bytes()...),
		}
		committee = append(committee, BeastDKGMember{Index: i, SigPub: append([]byte(nil), sigPub...), EncPub: append([]byte(nil), encPub.Bytes()...)})
	}

	bus := &memDKGBus{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runners := make([]*BeastDKGRunner, 0, n)
	for i := 1; i <= n; i++ {
		dir := t.TempDir()
		cfg := BeastDKGConfig{
			SessionID:    "sess",
			Epoch:        1,
			N:            n,
			Threshold:    k,
			Index:        i,
			KeySharePath: filepath.Join(dir, fmt.Sprintf("ks_%d.dat", i)),
			SigPriv:      nodeKeys[i].sigPriv,
			EncPriv:      nodeKeys[i].encPriv,
			Committee:    committee,
		}
		tr := &memDKGTransport{bus: bus}
		r, err := NewBeastDKGRunner(cfg, tr, WithRetryInterval(10*time.Millisecond))
		if err != nil {
			t.Fatalf("runner[%d]: %v", i, err)
		}
		if err := r.Start(ctx); err != nil {
			t.Fatalf("start[%d]: %v", i, err)
		}
		runners = append(runners, r)
	}

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		allDone := true
		for _, r := range runners {
			if _, ok := r.Result(); !ok {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		select {
		case <-deadline.C:
			t.Fatalf("timeout waiting for DKG complete")
		case <-tick.C:
		}
	}

	var gpk []byte
	shareList := make([]scalarShare, 0, n)
	for _, r := range runners {
		res, ok := r.Result()
		if !ok {
			t.Fatalf("missing result")
		}
		if len(gpk) == 0 {
			gpk = append([]byte(nil), res.GroupPubKey...)
		} else if string(gpk) != string(res.GroupPubKey) {
			t.Fatalf("group pubkey mismatch")
		}
		var sc blst.Scalar
		if sc.Deserialize(res.ShareScalar) == nil {
			t.Fatalf("bad share scalar")
		}
		shareList = append(shareList, scalarShare{Index: res.Index, Value: &sc})
	}

	secret, err := combineScalarSharesAtZero(shareList, k)
	if err != nil {
		t.Fatalf("combine: %v", err)
	}
	wantGPK := blst.P1Generator().Mult(secret).ToAffine().Compress()
	if string(wantGPK) != string(gpk) {
		t.Fatalf("reconstructed gpk mismatch")
	}
}

