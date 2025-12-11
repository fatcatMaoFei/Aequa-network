package payload

import (
	"errors"

	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// BuilderPolicy defines a deterministic selection strategy.
// Order lists payload types in priority (earlier first).
// MaxN caps total selected items.
// BudgetBytes is advisory (0 to ignore).
type BuilderPolicy struct {
	Order       []string
	MaxN        int
	BudgetBytes int
	MinBid      uint64
	MinFee      uint64
	Window      int
}

// PrepareProposal selects payloads from a container following the policy.
// It does not mutate the container; pools remain responsible for internal state.
func PrepareProposal(c *Container, hdr BlockHeader, pol BuilderPolicy) StandardBlock {
	max := pol.MaxN
	if max <= 0 {
		max = 1024
	}
	window := pol.Window
	if window <= 0 || window > max {
		window = max
	}
	res := make([]Payload, 0, max)
	remain := max
	for _, typ := range pol.Order {
		if remain <= 0 {
			break
		}
		// Enforce per-type window
		need := window
		if need > remain {
			need = remain
		}
		got := c.GetN(typ, need, pol.BudgetBytes)
		for _, p := range got {
			if len(res) >= max {
				break
			}
			if reject := belowThreshold(typ, p, pol); reject != "" {
				metrics.Inc("builder_reject_total", map[string]string{"type": typ, "reason": reject})
				continue
			}
			res = append(res, p)
			metrics.Inc("builder_selected_total", map[string]string{"type": typ})
		}
		remain = max - len(res)
	}
	return StandardBlock{Header: hdr, Items: res}
}

// ProcessProposal validates that a block complies with the deterministic policy.
// Checks:
// - Items only contain allowed types in policy
// - Type ordering obeys policy (all of a type appear before lower priority types)
// - Within the same type, SortKey is non-increasing
func ProcessProposal(b StandardBlock, pol BuilderPolicy) error {
	if len(pol.Order) == 0 {
		return nil
	}
	// build priority map
	pri := map[string]int{}
	for i, t := range pol.Order {
		pri[t] = i
	}
	lastPri := -1
	// track last SortKey per type to enforce non-increasing order
	lastKey := map[string]uint64{}
	for _, it := range b.Items {
		t := it.Type()
		p, ok := pri[t]
		if !ok {
			return errors.New("unexpected payload type: " + t)
		}
		if p < lastPri {
			return errors.New("type priority violated")
		}
		if prev, seen := lastKey[t]; seen {
			if it.SortKey() > prev {
				return errors.New("sortkey not non-increasing for type: " + t)
			}
		}
		lastKey[t] = it.SortKey()
		if p > lastPri {
			lastPri = p
		}
	}
	return nil
}

// belowThreshold returns reason if payload should be rejected for DFBA thresholds.
func belowThreshold(typ string, p Payload, pol BuilderPolicy) string {
	switch typ {
	case "auction_bid_v1":
		if pol.MinBid > 0 {
			if p.SortKey() < pol.MinBid {
				return "below_min_bid"
			}
		}
	case "plaintext_v1":
		if pol.MinFee > 0 {
			if p.SortKey() < pol.MinFee {
				return "below_min_fee"
			}
		}
	}
	return ""
}
