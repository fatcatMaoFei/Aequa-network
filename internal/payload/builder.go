package payload

import (
	"errors"
	"sort"
	"time"

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
	BatchTicks  int // optional time ticks per batch (for DFBA windowing), advisory
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
	now := time.Now()
	windowDur := time.Duration(pol.BatchTicks) * time.Millisecond
	for _, typ := range pol.Order {
		if remain <= 0 {
			break
		}
		// Enforce per-type window
		need := window
		if need > remain {
			need = remain
		}
		cands := c.GetAll(typ)
		filtered := make([]Payload, 0, len(cands))
		for _, p := range cands {
			if len(res) >= max {
				break
			}
			// Time window check if configured
			if windowDur > 0 {
				if meta, ok := c.Arrival(p); ok {
					if meta.TS.Before(now.Add(-windowDur)) {
						metrics.Inc("builder_reject_total", map[string]string{"type": typ, "reason": "late"})
						continue
					}
				}
			}
			if reject := belowThreshold(typ, p, pol); reject != "" {
				metrics.Inc("builder_reject_total", map[string]string{"type": typ, "reason": reject})
				continue
			}
			filtered = append(filtered, p)
		}
		// Sort by arrival seq asc (fairness), then SortKey desc as tie-breaker
		sort.SliceStable(filtered, func(i, j int) bool {
			mi, _ := c.Arrival(filtered[i])
			mj, _ := c.Arrival(filtered[j])
			if mi.Seq != mj.Seq {
				return mi.Seq < mj.Seq
			}
			return filtered[i].SortKey() > filtered[j].SortKey()
		})
		// enforce per-type window and remaining budget
		for _, p := range filtered {
			if len(res) >= max {
				break
			}
			if len(res) >= need {
				break
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
			// enforce non-increasing sort key per type (DFBA fairness)
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
