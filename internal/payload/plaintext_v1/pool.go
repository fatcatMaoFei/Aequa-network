package plaintext_v1

import (
	"crypto/sha256"
	"errors"
	"sort"
	"sync"

	"github.com/zmlAEQ/Aequa-network/internal/payload"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// PlaintextTx is a minimal, nonce-ordered tx used for stage-1 mempool.
type PlaintextTx struct{
    From  string
    Nonce uint64
    Gas   uint64
    Fee   uint64 // used as SortKey (higher first)
    Sig   []byte // shape-only validation in this stage
    h     []byte // cached hash
}

func (t *PlaintextTx) Type() string { return "plaintext_v1" }
func (t *PlaintextTx) Hash() []byte {
    if t.h == nil {
        sum := sha256.Sum256(append([]byte(t.From), byte(t.Nonce), byte(t.Gas), byte(t.Fee)))
        t.h = sum[:]
    }
    return t.h
}
func (t *PlaintextTx) Validate() error {
    if t.From == "" || t.Gas == 0 || len(t.Sig) < 32 { return errors.New("invalid") }
    return nil
}
func (t *PlaintextTx) SortKey() uint64 { return t.Fee }

// Pool implements a minimal pending/future nonce-ordered pool.
type Pool struct{
    mu      sync.Mutex
    // expected nonce per sender
    expect  map[string]uint64
    // pending ready list (per sender FIFO) and a flat slice for Get ordering
    pendBySender map[string][]*PlaintextTx
    // future holds txs with nonce > expected
    future  map[string]map[uint64]*PlaintextTx
}

func New() *Pool {
    return &Pool{expect: map[string]uint64{}, pendBySender: map[string][]*PlaintextTx{}, future: map[string]map[uint64]*PlaintextTx{}}
}

// Add inserts a payload; only PlaintextTx is accepted.
func (p *Pool) Add(pl payload.Payload) error {
    tx, ok := pl.(*PlaintextTx)
    if !ok || tx.Type() != "plaintext_v1" {
        return nil
    }
    if err := tx.Validate(); err != nil {
        metrics.Inc("mempool_in_total", map[string]string{"result":"invalid"})
        return err
    }
    p.mu.Lock(); defer p.mu.Unlock()
    exp := p.expect[tx.From]
    switch {
    case tx.Nonce < exp:
        metrics.Inc("mempool_in_total", map[string]string{"result":"old"})
        return errors.New("old nonce")
    case tx.Nonce == exp:
        p.pendBySender[tx.From] = append(p.pendBySender[tx.From], tx)
        p.expect[tx.From] = exp + 1
        // If subsequent futures become ready, promote them
        for {
            futs := p.future[tx.From]
            nx := p.expect[tx.From]
            if futs == nil { break }
            if next, ok := futs[nx]; ok {
                p.pendBySender[tx.From] = append(p.pendBySender[tx.From], next)
                delete(futs, nx)
                p.expect[tx.From] = nx + 1
            } else { break }
        }
        metrics.Inc("mempool_in_total", map[string]string{"result":"ok"})
        metrics.AddGauge("mempool_size", nil, 1)
        return nil
    default: // tx.Nonce > exp
        if p.future[tx.From] == nil { p.future[tx.From] = map[uint64]*PlaintextTx{} }
        // dedup future by (from, nonce)
        if _, exists := p.future[tx.From][tx.Nonce]; exists {
            metrics.Inc("mempool_in_total", map[string]string{"result":"dup"})
            return errors.New("duplicate future")
        }
        p.future[tx.From][tx.Nonce] = tx
        metrics.Inc("mempool_in_total", map[string]string{"result":"future"})
        return nil
    }
}

// Get returns up to n ready txs ordered by fee (desc), stable by (from,nonce).
func (p *Pool) Get(n int, _ int) []payload.Payload {
    p.mu.Lock(); defer p.mu.Unlock()
    // flatten
    buf := make([]*PlaintextTx, 0, 64)
    for from := range p.pendBySender {
        ll := p.pendBySender[from]
        buf = append(buf, ll...)
    }
    sort.SliceStable(buf, func(i, j int) bool { return buf[i].Fee > buf[j].Fee })
    if n > 0 && len(buf) > n { buf = buf[:n] }
    out := make([]payload.Payload, len(buf))
    for i, t := range buf { out[i] = t }
    return out
}

func (p *Pool) Len() int {
    p.mu.Lock(); defer p.mu.Unlock()
    sum := 0
    for _, ll := range p.pendBySender { sum += len(ll) }
    return sum
}
