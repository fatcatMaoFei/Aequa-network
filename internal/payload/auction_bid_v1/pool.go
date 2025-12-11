package auction_bid_v1

import (
	"crypto/sha256"
	"errors"
	"sort"
	"sync"

	"github.com/zmlAEQ/Aequa-network/internal/payload"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// AuctionBidTx represents a bid-style transaction used by DFBA.
// It sorts by Bid (descending) while keeping per-sender nonce order.
type AuctionBidTx struct {
	From         string
	Nonce        uint64
	Gas          uint64
	Bid          uint64 // used as SortKey (higher first)
	FeeRecipient string
	Sig          []byte // shape-only validation in this stage
	h            []byte // cached hash
}

func (t *AuctionBidTx) Type() string { return "auction_bid_v1" }

func (t *AuctionBidTx) Hash() []byte {
	if t.h == nil {
		sum := sha256.Sum256(append(append([]byte(t.From), []byte(t.FeeRecipient)...), byte(t.Nonce), byte(t.Gas), byte(t.Bid)))
		t.h = sum[:]
	}
	return t.h
}

func (t *AuctionBidTx) Validate() error {
	if t.From == "" || t.FeeRecipient == "" || t.Gas == 0 || t.Bid == 0 || len(t.Sig) < 32 {
		return errors.New("invalid")
	}
	return nil
}

func (t *AuctionBidTx) SortKey() uint64 { return t.Bid }

// Pool implements a minimal pending/future pool with bid-based ordering.
type Pool struct {
	mu           sync.Mutex
	expect       map[string]uint64
	pendBySender map[string][]*AuctionBidTx
	future       map[string]map[uint64]*AuctionBidTx
}

func New() *Pool {
	return &Pool{
		expect:       map[string]uint64{},
		pendBySender: map[string][]*AuctionBidTx{},
		future:       map[string]map[uint64]*AuctionBidTx{},
	}
}

// Add inserts a payload; only AuctionBidTx is accepted.
func (p *Pool) Add(pl payload.Payload) error {
	tx, ok := pl.(*AuctionBidTx)
	if !ok || tx.Type() != "auction_bid_v1" {
		return nil
	}
	if err := tx.Validate(); err != nil {
		metrics.Inc("mempool_in_total", map[string]string{"result": "invalid"})
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	exp := p.expect[tx.From]
	switch {
	case tx.Nonce < exp:
		metrics.Inc("mempool_in_total", map[string]string{"result": "old"})
		return errors.New("old nonce")
	case tx.Nonce == exp:
		p.pendBySender[tx.From] = append(p.pendBySender[tx.From], tx)
		p.expect[tx.From] = exp + 1
		for {
			futs := p.future[tx.From]
			nx := p.expect[tx.From]
			if futs == nil {
				break
			}
			if next, ok := futs[nx]; ok {
				p.pendBySender[tx.From] = append(p.pendBySender[tx.From], next)
				delete(futs, nx)
				p.expect[tx.From] = nx + 1
			} else {
				break
			}
		}
		metrics.Inc("mempool_in_total", map[string]string{"result": "ok"})
		metrics.AddGauge("mempool_size", nil, 1)
		return nil
	default:
		if p.future[tx.From] == nil {
			p.future[tx.From] = map[uint64]*AuctionBidTx{}
		}
		if _, exists := p.future[tx.From][tx.Nonce]; exists {
			metrics.Inc("mempool_in_total", map[string]string{"result": "dup"})
			return errors.New("duplicate future")
		}
		p.future[tx.From][tx.Nonce] = tx
		metrics.Inc("mempool_in_total", map[string]string{"result": "future"})
		return nil
	}
}

// Get returns up to n ready txs ordered by bid (desc), stable by (from,nonce).
func (p *Pool) Get(n int, _ int) []payload.Payload {
	p.mu.Lock()
	defer p.mu.Unlock()
	buf := make([]*AuctionBidTx, 0, 64)
	for _, ll := range p.pendBySender {
		buf = append(buf, ll...)
	}
	sort.SliceStable(buf, func(i, j int) bool { return buf[i].Bid > buf[j].Bid })
	if n > 0 && len(buf) > n {
		buf = buf[:n]
	}
	out := make([]payload.Payload, len(buf))
	for i, t := range buf {
		out[i] = t
	}
	return out
}

func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	sum := 0
	for _, ll := range p.pendBySender {
		sum += len(ll)
	}
	return sum
}
