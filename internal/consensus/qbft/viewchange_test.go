package qbft

import (
    "strings"
    "testing"

    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func TestViewChange_AdvanceOnThreshold(t *testing.T) {
    metrics.Reset()
    st := &State{Leader: "L"}
    // timeout on leader -> produce local view-change targeting round 1
    _ = st.Process(Message{ID:"blk", From:"L", Type:MsgPreprepare, Height: 30, Round:0})
    vc := st.OnTimeout()
    if vc.Type != MsgViewChange || vc.Round != 1 { t.Fatalf("bad vc: %+v", vc) }
    // collect two distinct view-change votes for round 1
    if err := st.Process(Message{ID:"vc1", From:"P1", Type:MsgViewChange, Height: 30, Round:1}); err != nil { t.Fatalf("vc1: %v", err) }
    if st.Round != 0 { t.Fatalf("round advanced too early: %d", st.Round) }
    if err := st.Process(Message{ID:"vc2", From:"P2", Type:MsgViewChange, Height: 30, Round:1}); err != nil { t.Fatalf("vc2: %v", err) }
    if st.Round != 1 { t.Fatalf("want round=1 after VC threshold, got %d", st.Round) }
    dump := metrics.DumpProm()
    if !strings.Contains(dump, `qbft_view_changes_total`) { t.Fatalf("missing view change counter in %q", dump) }
    if !strings.Contains(dump, `qbft_timeouts_total{phase="preprepared"} 1`) && !strings.Contains(dump, `qbft_timeouts_total{phase="idle"} 1`) {
        t.Fatalf("missing timeout counter in %q", dump)
    }
}

func TestViewChange_DuplicatesNoDoubleAdvance(t *testing.T) {
    metrics.Reset()
    st := &State{}
    // two votes from the same source should count once
    _ = st.Process(Message{ID:"v1", From:"A", Type:MsgViewChange, Round:1})
    _ = st.Process(Message{ID:"v1dup", From:"A", Type:MsgViewChange, Round:1})
    if st.Round != 0 { t.Fatalf("should not advance with single source, got %d", st.Round) }
    _ = st.Process(Message{ID:"v2", From:"B", Type:MsgViewChange, Round:1})
    if st.Round != 1 { t.Fatalf("want advance to 1, got %d", st.Round) }
    // another duplicate should not increment counter again
    dump := metrics.DumpProm()
    if !strings.Contains(dump, `qbft_view_changes_total`) { t.Fatalf("missing view change counter in %q", dump) }
}

