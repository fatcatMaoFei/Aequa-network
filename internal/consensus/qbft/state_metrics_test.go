package qbft

import (
    "strings"
    "testing"

    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// Ensure new metric families tick exactly once when thresholds are reached
// and do not drift with duplicates/extra votes.
func TestState_Metrics_QC_And_Msg_Count(t *testing.T) {
    metrics.Reset()
    st := &State{Leader: "L"}

    // preprepare -> enter preprepared
    if err := st.Process(Message{ID:"blkM", From:"L", Type:MsgPreprepare, Height: 20, Round:0}); err != nil {
        t.Fatalf("preprepare: %v", err)
    }
    // two distinct prepares -> prepared (QC:prepare increments once)
    if err := st.Process(Message{ID:"blkM", From:"P1", Type:MsgPrepare, Height: 20, Round:1}); err != nil { t.Fatalf("p1: %v", err) }
    if err := st.Process(Message{ID:"blkM", From:"P2", Type:MsgPrepare, Height: 20, Round:1}); err != nil { t.Fatalf("p2: %v", err) }
    // duplicate prepare should not increment QC again
    if err := st.Process(Message{ID:"blkM", From:"P2", Type:MsgPrepare, Height: 20, Round:1}); err != nil { t.Fatalf("dup: %v", err) }

    // one commit -> commit (QC:commit increments once)
    if err := st.Process(Message{ID:"blkM", From:"C1", Type:MsgCommit, Height: 20, Round:1}); err != nil { t.Fatalf("commit: %v", err) }
    // duplicate commit should not increment QC again
    if err := st.Process(Message{ID:"blkM", From:"C1", Type:MsgCommit, Height: 20, Round:1}); err != nil { t.Fatalf("dup commit: %v", err) }

    dump := metrics.DumpProm()
    // qbft_msg_total should record at least one of each type we used
    for _, typ := range []string{"preprepare","prepare","commit"} {
        if !strings.Contains(dump, `qbft_msg_total{type="`+typ+`"}`) {
            t.Fatalf("missing qbft_msg_total for type %s in %q", typ, dump)
        }
    }
    // QC counters should be exactly 1
    if !strings.Contains(dump, `qbft_qc_built_total{kind="prepare"} 1`) {
        t.Fatalf("want prepare QC =1, got %q", dump)
    }
    if !strings.Contains(dump, `qbft_qc_built_total{kind="commit"} 1`) {
        t.Fatalf("want commit QC =1, got %q", dump)
    }
}

