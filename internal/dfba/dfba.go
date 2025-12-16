package dfba

import (
	"sort"
	"time"

	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// Item carries the minimal fields needed by the DFBA solver while keeping
// the payload type abstract to avoid import cycles with concrete payload
// implementations.
type Item struct {
	Payload any
	Type    string
	Key     uint64
	Hash    []byte
}

// Policy mirrors the DFBA-related fields from payload.BuilderPolicy that are
// relevant for deterministic selection.
type Policy struct {
	Order      []string
	MaxN       int
	MinBid     uint64
	MinFee     uint64
	Window     int
	BatchTicks int
}

// SolverInput is the top-level input to SolveDeterministic.
type SolverInput struct {
	Items  []Item
	Policy Policy
}

// Result holds the subset of items selected in deterministic order.
type Result struct {
	Selected []Item
}

// SolveDeterministic applies a dual-flow batch auction style selection:
// - It treats "auction_bid_v1" as solver flow and "plaintext_v1" as user
//   flow when both are present.
// - It finds a matched cardinality k bounded by Policy.MaxN, Policy.Window
//   and the two flow sizes, and selects the top‑k bids and top‑k user
//   txs under their respective sort keys.
// - When one of the flows is empty or no capacity remains for a full pair,
//   it falls back to per-type windowed selection that mirrors the existing
//   builder behaviour.
func SolveDeterministic(in SolverInput) (Result, error) {
	start := time.Now()
	defer func() {
		durMs := time.Since(start).Milliseconds()
		metrics.ObserveSummary("dfba_solve_ms", nil, float64(durMs))
	}()

	max := in.Policy.MaxN
	if max <= 0 {
		max = 1024
	}
	window := in.Policy.Window
	if window <= 0 || window > max {
		window = max
	}

	// Index items by type.
	byType := make(map[string][]Item)
	for _, it := range in.Items {
		byType[it.Type] = append(byType[it.Type], it)
	}

	// Fast-path: if either flow is missing, fall back to per-type windowed selection.
	bids := make([]Item, len(byType["auction_bid_v1"]))
	copy(bids, byType["auction_bid_v1"])
	users := make([]Item, len(byType["plaintext_v1"]))
	copy(users, byType["plaintext_v1"])

	if len(bids) == 0 || len(users) == 0 {
		selected := selectPerType(byType, in.Policy.Order, max, window)
		metrics.Inc("dfba_solve_total", map[string]string{"result": "fallback"})
		return Result{Selected: selected}, nil
	}

	// Sort flows by value (Key desc, Hash tie‑breaker) to keep deterministic.
	sort.SliceStable(bids, func(i, j int) bool { return lessByKeyHash(bids[i], bids[j]) })
	sort.SliceStable(users, func(i, j int) bool { return lessByKeyHash(users[i], users[j]) })

	// Determine match cardinality: each match consumes one bid + one user,
	// i.e. two items of capacity.
	pairsCap := max / 2
	if pairsCap <= 0 {
		selected := selectPerType(byType, in.Policy.Order, max, window)
		metrics.Inc("dfba_solve_total", map[string]string{"result": "fallback"})
		return Result{Selected: selected}, nil
	}
	k := len(bids)
	if len(users) < k {
		k = len(users)
	}
	if k > window {
		k = window
	}
	if k > pairsCap {
		k = pairsCap
	}
	if k <= 0 {
		selected := selectPerType(byType, in.Policy.Order, max, window)
		metrics.Inc("dfba_solve_total", map[string]string{"result": "fallback"})
		return Result{Selected: selected}, nil
	}

	// Build selection grouped by type according to policy order.
	selected := make([]Item, 0, 2*k)
	for _, typ := range in.Policy.Order {
		switch typ {
		case "auction_bid_v1":
			selected = append(selected, bids[:k]...)
		case "plaintext_v1":
			selected = append(selected, users[:k]...)
		default:
			// other types: keep behaviour consistent with previous DFBA skeleton
			list := byType[typ]
			if len(list) == 0 {
				continue
			}
			sort.SliceStable(list, func(i, j int) bool { return lessByKeyHash(list[i], list[j]) })
			need := window
			if need > max-len(selected) {
				need = max - len(selected)
			}
			if need > len(list) {
				need = len(list)
			}
			if need > 0 {
				selected = append(selected, list[:need]...)
			}
		}
	}

	metrics.Inc("dfba_solve_total", map[string]string{"result": "ok"})
	return Result{Selected: selected}, nil
}

// lessByKeyHash orders two items by Key desc, then Hash asc.
func lessByKeyHash(a, b Item) bool {
	if a.Key != b.Key {
		return a.Key > b.Key
	}
	ha := a.Hash
	hb := b.Hash
	if len(ha) == len(hb) {
		for i := range ha {
			if ha[i] != hb[i] {
				return ha[i] < hb[i]
			}
		}
	}
	return false
}

// selectPerType mirrors the previous deterministic builder behaviour: walk
// types in order, sort within each type by value, then take up to a per-type
// window while respecting a global cap.
func selectPerType(byType map[string][]Item, order []string, max, window int) []Item {
	selected := make([]Item, 0, max)
	remain := max
	for _, typ := range order {
		if remain <= 0 {
			break
		}
		list := byType[typ]
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool { return lessByKeyHash(list[i], list[j]) })
		need := window
		if need > remain {
			need = remain
		}
		if need > len(list) {
			need = len(list)
		}
		if need > 0 {
			selected = append(selected, list[:need]...)
			remain = max - len(selected)
		}
	}
	return selected
}
