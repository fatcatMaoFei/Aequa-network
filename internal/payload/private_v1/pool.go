package private_v1

import (
	"crypto/sha256"
	"errors"
	"sync"

	"github.com/zmlAEQ/Aequa-network/internal/payload"
)

// PrivateTx represents a BEAST-style encrypted transaction (stub).
// Sorting is not used; SortKey returns 0.
type PrivateTx struct {
	From         string
	Nonce        uint64
	Ciphertext   []byte
	EphemeralKey []byte
	TargetHeight uint64
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

// Pool is a stub mempool for private transactions; it accepts all valid txs.
type Pool struct {
	mu    sync.Mutex
	items []payload.Payload
}

func New() *Pool { return &Pool{} }

func (p *Pool) Add(pl payload.Payload) error {
	if tx, ok := pl.(*PrivateTx); ok {
		if err := tx.Validate(); err != nil {
			return err
		}
		p.mu.Lock()
		p.items = append(p.items, tx)
		p.mu.Unlock()
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
