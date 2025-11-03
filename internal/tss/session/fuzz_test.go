package session

import (
    "context"
    "testing"
    "time"
)

// FuzzManager_NoPanic mixes shares and timeouts ensuring no panics and phase
// stays within known labels.
func FuzzManager_NoPanic(f *testing.F) {
    f.Add(uint8(0), uint8(0), uint8(0))
    f.Add(uint8(1), uint8(1), uint8(1))
    f.Fuzz(func(t *testing.T, a, b, c uint8) {
        m := NewManager(Config{Threshold: 2, GatherTimeout: 5 * time.Millisecond})
        m.Start(context.Background())
        // first share (enter gather)
        _, _ = m.OnShare("P1", "n1", nil)
        if a%2 == 0 {
            // second distinct share → combine
            _, _ = m.OnShare("P2", "n2", nil)
            if b%2 == 0 {
                m.Finalize()
            }
        } else {
            // duplicate share (no-op)
            _, _ = m.OnShare("P1", "n1", nil)
        }
        if c%2 == 1 {
            time.Sleep(7 * time.Millisecond) // trigger possible timeout path
        }
        st := m.Status()
        switch st.Phase {
        case PhaseInit, PhaseGather, PhaseCombine, PhaseDone:
            // ok
        default:
            t.Fatalf("unknown phase: %s", st.Phase)
        }
        m.Stop()
    })
}

