package private_v1

import (
	"crypto/sha256"
	"errors"
	"os"
	"sync"

	"github.com/zmlAEQ/Aequa-network/internal/payload"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// PrivateTx represents a BEAST-style encrypted transaction (stub).
// Sorting is not used; SortKey returns 0.
type PrivateTx struct {
	From         string
	Nonce        uint64
	Ciphertext   []byte
	EphemeralKey []byte
	TargetHeight uint64
	// Optional batched BEAST fields (used when Mode=="batched").
	BatchIndex   uint64
	PuncturedKey []byte
	h            []byte // cached hash
}

func (t *PrivateTx) Type() string { return "private_v1" }

func (t *PrivateTx) Hash() []byte {
	if t.h == nil {
		sum := sha256.Sum256(append(append([]byte(t.From), byte(t.Nonce)), t.Ciphertext...))
		t.h = sum[:]
	}
	return t.h
}

func (t *PrivateTx) Validate() error {
	if t.From == "" || len(t.Ciphertext) == 0 || len(t.EphemeralKey) == 0 {
		return errors.New("invalid")
	}
	return nil
}

func (t *PrivateTx) SortKey() uint64 { return 0 }

// Pool is a stub mempool for private transactions with basic capacity and
// duplicate guards to avoid unbounded growth.
type Pool struct {
	mu    sync.Mutex
	items []payload.Payload
	seen  map[string]struct{}
	max   int
}

func New() *Pool { return &Pool{seen: map[string]struct{}{}, max: 4096} }

func (p *Pool) Add(pl payload.Payload) error {
	if tx, ok := pl.(*PrivateTx); ok {
		if err := tx.Validate(); err != nil {
			metrics.Inc("private_pool_in_total", map[string]string{"result": "invalid"})
			return err
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		if len(p.items) >= p.max {
			metrics.Inc("private_pool_in_total", map[string]string{"result": "overflow"})
			return errors.New("private pool overflow")
		}
		h := string(tx.Hash())
		if p.seen == nil {
			p.seen = map[string]struct{}{}
		}
		if _, exists := p.seen[h]; exists {
			metrics.Inc("private_pool_in_total", map[string]string{"result": "dup"})
			return errors.New("duplicate private tx")
		}
		p.items = append(p.items, tx)
		p.seen[h] = struct{}{}
		metrics.Inc("private_pool_in_total", map[string]string{"result": "ok"})
		metrics.SetGauge("private_pool_size", nil, int64(len(p.items)))
		// Optional (dev-only): precompute/publish threshold share at ingest.
		// Disabled by default because it can leak decrypt shares before TargetHeight.
		if os.Getenv("AEQUA_BEAST_EARLY_SHARE") == "1" {
			maybeEnsureShare(tx.TargetHeight)
		}
		return nil
	}
	return nil
}

func (p *Pool) Get(n int, _ int) []payload.Payload {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n <= 0 || n > len(p.items) {
		n = len(p.items)
	}
	out := make([]payload.Payload, n)
	copy(out, p.items[:n])
	return out
}
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.items)
}
