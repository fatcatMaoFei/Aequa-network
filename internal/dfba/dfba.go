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

// SolveDeterministic applies a simple deterministic selection strategy:
// it enforces per-type ordering according to Policy.Order, caps total
// items by MaxN, and limits per-type contribution by Window. Within the
// same type, items are sorted by Key (desc) with Hash as a tie-breaker
// to keep ordering stable.
//
// This is a structural entry-point for DFBA; initial implementation
// preserves existing builder semantics and is guarded behind a flag.
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

	// Index items by type for per-type windowing.
	byType := make(map[string][]Item)
	for _, it := range in.Items {
		byType[it.Type] = append(byType[it.Type], it)
	}

	selected := make([]Item, 0, max)
	remain := max
	for _, typ := range in.Policy.Order {
		if remain <= 0 {
			break
		}
		list := byType[typ]
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Key != list[j].Key {
				return list[i].Key > list[j].Key
			}
			hi := list[i].Hash
			hj := list[j].Hash
			if len(hi) == len(hj) {
				for k := range hi {
					if hi[k] != hj[k] {
						return hi[k] < hj[k]
					}
				}
			}
			return false
		})
		need := window
		if need > remain {
			need = remain
		}
		if need > len(list) {
			need = len(list)
		}
		selected = append(selected, list[:need]...)
		remain = max - len(selected)
	}

	metrics.Inc("dfba_solve_total", map[string]string{"result": "ok"})
	return Result{Selected: selected}, nil
}

