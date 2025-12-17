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
	"sync/atomic"
	"testing"
	"time"

	"github.com/zmlAEQ/Aequa-network/internal/p2p/wire"

	blst "github.com/supranational/blst/bindings/go"
)

type memDKGBus struct {
	mu   sync.Mutex
	subs []func(wire.TSSDKG)

	shareOpen  int64
	complaints int64
}

func (b *memDKGBus) subscribe(fn func(wire.TSSDKG)) {
	b.mu.Lock()
	b.subs = append(b.subs, fn)
	b.mu.Unlock()
}

func (b *memDKGBus) broadcast(m wire.TSSDKG) {
	switch m.Type {
	case "share_open":
		atomic.AddInt64(&b.shareOpen, 1)
	case "complaint":
		atomic.AddInt64(&b.complaints, 1)
	}
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

func TestBeastDKGRunner_ComplaintShareOpen_Recovers(t *testing.T) {
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
		// Simulate a receiver with mismatched enc_priv: it cannot decrypt encrypted shares,
		// but can recover via share_open complaint resolution.
		if i == 2 {
			bad := make([]byte, 32)
			_, _ = rand.Read(bad)
			r.cfg.EncPriv = bad
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

	if atomic.LoadInt64(&bus.shareOpen) == 0 {
		t.Fatalf("expected share_open messages")
	}
	if atomic.LoadInt64(&bus.complaints) == 0 {
		t.Fatalf("expected complaint messages")
	}
}

func TestBeastDKGRunner_DisqualifiesBadDealer(t *testing.T) {
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

func TestBeastDKGRunner_AdoptsHigherEpoch(t *testing.T) {
	const (
		n = 2
		k = 2
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

	// Node 2 starts on epoch 1; node 1 starts on epoch 2 and should cause node 2 to adopt.
	dir2 := t.TempDir()
	cfg2 := BeastDKGConfig{
		SessionID:    "sess",
		Epoch:        1,
		N:            n,
		Threshold:    k,
		Index:        2,
		KeySharePath: filepath.Join(dir2, "ks_2.dat"),
		SigPriv:      nodeKeys[2].sigPriv,
		EncPriv:      nodeKeys[2].encPriv,
		Committee:    committee,
	}
	r2, err := NewBeastDKGRunner(cfg2, &memDKGTransport{bus: bus}, WithRetryInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("runner[2]: %v", err)
	}
	if err := r2.Start(ctx); err != nil {
		t.Fatalf("start[2]: %v", err)
	}

	dir1 := t.TempDir()
	cfg1 := BeastDKGConfig{
		SessionID:    "sess",
		Epoch:        2,
		N:            n,
		Threshold:    k,
		Index:        1,
		KeySharePath: filepath.Join(dir1, "ks_1.dat"),
		SigPriv:      nodeKeys[1].sigPriv,
		EncPriv:      nodeKeys[1].encPriv,
		Committee:    committee,
	}
	r1, err := NewBeastDKGRunner(cfg1, &memDKGTransport{bus: bus}, WithRetryInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("runner[1]: %v", err)
	}
	if err := r1.Start(ctx); err != nil {
		t.Fatalf("start[1]: %v", err)
	}

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		_, ok1 := r1.Result()
		_, ok2 := r2.Result()
		if ok1 && ok2 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatalf("timeout waiting for epoch adoption")
		case <-tick.C:
		}
	}

	if r2.epoch != 2 {
		t.Fatalf("node2 did not adopt epoch 2: got=%d", r2.epoch)
	}
}

	bus := &memDKGBus{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start dealer 1 first, then mutate its local polynomial so future shares don't match commitments.
	{
		dir := t.TempDir()
		cfg := BeastDKGConfig{
			SessionID:    "sess",
			Epoch:        1,
			N:            n,
			Threshold:    k,
			Index:        1,
			KeySharePath: filepath.Join(dir, "ks_1.dat"),
			SigPriv:      nodeKeys[1].sigPriv,
			EncPriv:      nodeKeys[1].encPriv,
			Committee:    committee,
		}
		r1, err := NewBeastDKGRunner(cfg, &memDKGTransport{bus: bus}, WithRetryInterval(10*time.Millisecond))
		if err != nil {
			t.Fatalf("runner[1]: %v", err)
		}
		if err := r1.Start(ctx); err != nil {
			t.Fatalf("start[1]: %v", err)
		}
		// Malicious: replace coefficients after commitments are broadcast/persisted.
		r1.mu.Lock()
		bad := make([]*blst.Scalar, 0, k)
		for i := 0; i < k; i++ {
			sc, err := randScalar(rand.Reader)
			if err != nil {
				r1.mu.Unlock()
				t.Fatalf("randScalar: %v", err)
			}
			bad = append(bad, sc)
		}
		r1.coeffs = bad
		r1.mu.Unlock()
	}

	runners := make([]*BeastDKGRunner, 0, n-1)
	for idx := 2; idx <= n; idx++ {
		dir := t.TempDir()
		cfg := BeastDKGConfig{
			SessionID:    "sess",
			Epoch:        1,
			N:            n,
			Threshold:    k,
			Index:        idx,
			KeySharePath: filepath.Join(dir, fmt.Sprintf("ks_%d.dat", idx)),
			SigPriv:      nodeKeys[idx].sigPriv,
			EncPriv:      nodeKeys[idx].encPriv,
			Committee:    committee,
		}
		r, err := NewBeastDKGRunner(cfg, &memDKGTransport{bus: bus}, WithRetryInterval(10*time.Millisecond))
		if err != nil {
			t.Fatalf("runner[%d]: %v", idx, err)
		}
		if err := r.Start(ctx); err != nil {
			t.Fatalf("start[%d]: %v", idx, err)
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
			t.Fatalf("timeout waiting for honest nodes to complete")
		case <-tick.C:
		}
	}

	if atomic.LoadInt64(&bus.shareOpen) == 0 || atomic.LoadInt64(&bus.complaints) == 0 {
		t.Fatalf("expected complaint/share_open activity")
	}
}
