package payload

import (
	"errors"
	"os"
	"sort"
	"time"

	"github.com/zmlAEQ/Aequa-network/internal/dfba"
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
	BatchTicks  int  // optional time ticks per batch (for DFBA windowing), advisory
	UseDFBA     bool // when true, route selection through DFBA solver (behind flag)
}

// PrepareProposal selects payloads from a container following the policy.
// It does not mutate the container; pools remain responsible for internal state.
func PrepareProposal(c *Container, hdr BlockHeader, pol BuilderPolicy) StandardBlock {
	if pol.UseDFBA {
		return prepareProposalDFBA(c, hdr, pol)
	}
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
		need := window
		if need > remain {
			need = remain
		}
		cands := c.GetAll(typ)
		filtered := filterByWindowAndThreshold(c, cands, typ, pol, now, windowDur)
		if typ == "private_v1" && os.Getenv("AEQUA_ENABLE_BEAST") == "1" {
			filtered = decryptAndMapPrivate(filtered)
		}
		selected := takeDeterministic(filtered, need, max-len(res))
		res = append(res, selected...)
		for i := 0; i < len(selected); i++ {
			metrics.Inc("builder_selected_total", map[string]string{"type": typ})
		}
		remain = max - len(res)
	}
	return StandardBlock{Header: hdr, Items: res}
}

// prepareProposalDFBA routes selection through the DFBA solver when enabled.
// It preserves existing filtering semantics (time window + thresholds) and
// uses DFBA only for per-type windowed ordering and capping.
func prepareProposalDFBA(c *Container, hdr BlockHeader, pol BuilderPolicy) StandardBlock {
	max := pol.MaxN
	if max <= 0 {
		max = 1024
	}
	window := pol.Window
	if window <= 0 || window > max {
		window = max
	}
	now := time.Now()
	windowDur := time.Duration(pol.BatchTicks) * time.Millisecond
	items := make([]dfba.Item, 0, max)
	all := make([]dfba.Item, 0, max)
	for _, typ := range pol.Order {
		cands := c.GetAll(typ)
		filtered := filterByWindowAndThreshold(c, cands, typ, pol, now, windowDur)
		if typ == "private_v1" && os.Getenv("AEQUA_ENABLE_BEAST") == "1" {
			filtered = decryptAndMapPrivate(filtered)
		}
		for _, p := range filtered {
			if p == nil {
				continue
			}
			it := dfba.Item{
				Payload: p,
				Type:    typ,
				Key:     p.SortKey(),
				Hash:    p.Hash(),
			}
			// all holds every candidate that passed local filters
			all = append(all, it)
			items = append(items, it)
		}
	}
	dfbaPol := dfba.Policy{
		Order:      pol.Order,
		MaxN:       max,
		MinBid:     pol.MinBid,
		MinFee:     pol.MinFee,
		Window:     window,
		BatchTicks: pol.BatchTicks,
	}
	out, _ := dfba.SolveDeterministic(dfba.SolverInput{Items: items, Policy: dfbaPol})
	selectedSet := map[string]struct{}{}
	for _, it := range out.Selected {
		h := string(it.Hash)
		selectedSet[h] = struct{}{}
	}
	res := make([]Payload, 0, len(out.Selected))
	for _, it := range out.Selected {
		if plAny, ok := it.Payload.(Payload); ok && plAny != nil {
			res = append(res, plAny)
			metrics.Inc("builder_selected_total", map[string]string{"type": it.Type})
		}
	}
	// mark DFBA-specific drops for observability; reuse existing builder_reject_total
	for _, it := range all {
		h := string(it.Hash)
		if _, ok := selectedSet[h]; !ok {
			metrics.Inc("builder_reject_total", map[string]string{"type": it.Type, "reason": "dfba_no_match"})
		}
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

// filterByWindowAndThreshold applies window time check and thresholds.
func filterByWindowAndThreshold(c *Container, cands []Payload, typ string, pol BuilderPolicy, now time.Time, windowDur time.Duration) []Payload {
	filtered := make([]Payload, 0, len(cands))
	for _, p := range cands {
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
	return filtered
}

// decryptAndMapPrivate: placeholder for BEAST decrypt + mapping into sortable payload.
func decryptAndMapPrivate(cands []Payload) []Payload {
	// TODO: integrate beast decrypt and mapping to plaintext/auction payloads.
	// For now, use the registered decrypter (noop by default) to keep the path wired.
	out := make([]Payload, 0, len(cands))
	for _, p := range cands {
		if p == nil {
			continue
		}
		dec, err := privateDecrypter.Decrypt(p)
		if err != nil || dec == nil {
			recordDecryptMetric("error")
			continue
		}
		out = append(out, dec)
	}
	if len(out) == 0 {
		recordDecryptMetric("empty")
	} else {
		recordDecryptMetric("ok")
	}
	return out
}

// takeDeterministic sorts by arrival seq asc, then SortKey desc, and takes up to need, respecting total budget.
func takeDeterministic(cands []Payload, need int, budget int) []Payload {
	if need > budget {
		need = budget
	}
	sort.SliceStable(cands, func(i, j int) bool {
		ci := cands[i].Hash()
		cj := cands[j].Hash()
		// keep deterministic ordering based on hash as tie-breaker for stable sort
		if len(ci) == len(cj) {
			for k := range ci {
				if ci[k] != cj[k] {
					return ci[k] < cj[k]
				}
			}
		}
		return cands[i].SortKey() > cands[j].SortKey()
	})
	if need <= 0 || need > len(cands) {
		need = len(cands)
	}
	return cands[:need]
}
