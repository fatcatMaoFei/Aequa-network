package session

import (
    "context"
    "testing"
    "time"
)

func TestManager_GatherToCombine_AfterThreshold(t *testing.T) {
    m := NewManager(Config{Threshold: 2, GatherTimeout: time.Second})
    m.Start(context.Background())
    if adv, _ := m.OnShare("P1", "n1", nil); adv { t.Fatalf("should not advance on first share") }
    st := m.Status(); if st.Phase != PhaseGather { t.Fatalf("phase=%s", st.Phase) }
    if adv, _ := m.OnShare("P2", "n2", nil); !adv { t.Fatalf("should advance to combine on second distinct share") }
    st = m.Status(); if st.Phase != PhaseCombine { t.Fatalf("phase=%s", st.Phase) }
    m.Finalize()
    st = m.Status(); if st.Phase != PhaseDone { t.Fatalf("phase=%s", st.Phase) }
    m.Stop()
}

func TestManager_Dedup_NoAdvance(t *testing.T) {
    m := NewManager(Config{Threshold: 2, GatherTimeout: time.Second})
    m.Start(context.Background())
    _, _ = m.OnShare("P1", "n", nil)
    // duplicate from same from|nonce
    if adv, _ := m.OnShare("P1", "n", nil); adv { t.Fatalf("duplicate should not advance") }
    st := m.Status(); if st.Phase != PhaseGather { t.Fatalf("phase=%s", st.Phase) }
    // distinct share triggers combine
    if adv, _ := m.OnShare("P2", "n2", nil); !adv { t.Fatalf("second distinct should advance") }
    m.Stop()
}

func TestManager_Timeout_InGather(t *testing.T) {
    m := NewManager(Config{Threshold: 2, GatherTimeout: 30 * time.Millisecond})
    m.Start(context.Background())
    // enter gather with one share, then wait for timeout
    _, _ = m.OnShare("P1", "n1", nil)
    time.Sleep(60 * time.Millisecond)
    st := m.Status()
    if !st.TimedOut || st.Phase != PhaseDone { t.Fatalf("want timeout->done, got timedOut=%v phase=%s", st.TimedOut, st.Phase) }
    m.Stop()
}

