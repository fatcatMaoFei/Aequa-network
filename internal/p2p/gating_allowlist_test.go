package p2p

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func TestAllowListGate_ServiceAttempts(t *testing.T) {
	metrics.Reset()
	g := NewAllowListGate("A")
	s := NewWithOpts(nil, g, NewResourceManager(ResourceLimits{MaxConns: 2}), NopHook{})

	if err := s.Connect("A"); err != nil {
		t.Fatalf("A should pass: %v", err)
	}
	if err := s.Connect("B"); err == nil {
		t.Fatalf("B should be denied")
	}

	dump := metrics.DumpProm()
	if !strings.Contains(dump, `p2p_conn_attempts_total{result="allowed"} 1`) {
		t.Fatalf("expected allowed=1, got %q", dump)
	}
	if !strings.Contains(dump, `p2p_conn_attempts_total{result="denied"} 1`) {
		t.Fatalf("expected denied=1, got %q", dump)
	}
}

// TestAllowListGate_ConcurrentUpdates stresses Add/Remove concurrently with Connect attempts.
// It asserts: (1) no panic; (2) final state correctness; (3) metrics allowed+denied equals attempts.
func TestAllowListGate_ConcurrentUpdates(t *testing.T) {
	metrics.Reset()
	g := NewAllowListGate("A")
	r := NewResourceManager(ResourceLimits{MaxConns: 1024})
	s := NewWithOpts(nil, g, r, NopHook{})

	// Toggle B in allowlist while issuing Connect(B) in parallel.
	peer := PeerID("B")
	togglers := 4
	togglesPer := 200
	attempters := 8
	attemptsPer := 200

	var wg sync.WaitGroup
	// Toggling goroutines.
	wg.Add(togglers)
	for i := 0; i < togglers; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < togglesPer; j++ {
				if (i+j)%2 == 0 {
					g.Add(peer)
				} else {
					g.Remove(peer)
				}
				// small yield to increase interleaving
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Connect attempt goroutines.
	// Includes the two deterministic Connect calls after the goroutines finish.
	totalAttempts := attempters*attemptsPer + 2
	wg.Add(attempters)
	for i := 0; i < attempters; i++ {
		go func() {
			defer wg.Done()
			for k := 0; k < attemptsPer; k++ {
				if err := s.Connect(peer); err == nil {
					// release resource to avoid hitting MaxConns
					s.Disconnect(peer)
				}
			}
		}()
	}
	wg.Wait()

	// Final deterministic state checks: remove then add and assert behavior.
	g.Remove(peer)
	if err := s.Connect(peer); err == nil {
		t.Fatalf("expected denied after final remove")
	}
	g.Add(peer)
	if err := s.Connect(peer); err != nil {
		t.Fatalf("expected allowed after final add: %v", err)
	}
	s.Disconnect(peer)

	// Metrics invariants: allowed + denied equals total attempts (no limited/dkg_denied expected).
	dump := metrics.DumpProm()
	// helper to extract a single counter value from prom text lines.
	extract := func(label string) int {
		target := fmt.Sprintf("p2p_conn_attempts_total{result=\"%s\"}", label)
		for _, line := range strings.Split(dump, "\n") {
			if strings.HasPrefix(line, target+" ") {
				var v int
				_, _ = fmt.Sscanf(line, target+" %d", &v)
				return v
			}
		}
		return 0
	}
	allowed := extract("allowed")
	denied := extract("denied")
	if allowed+denied != totalAttempts {
		t.Fatalf("attempts mismatch: allowed(%d)+denied(%d) != %d; dump=%q", allowed, denied, totalAttempts, dump)
	}
}
