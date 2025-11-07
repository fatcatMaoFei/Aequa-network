package consensus

import (
    "context"
    "sync/atomic"
    "testing"
    "time"
    "path/filepath"

    "github.com/zmlAEQ/Aequa-network/pkg/bus"
    qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
    "github.com/zmlAEQ/Aequa-network/internal/state"
)

// stub store to count saves
type guardStore struct{ saves int32 }
func (s *guardStore) SaveLastState(_ context.Context, ls state.LastState) error { atomic.AddInt32(&s.saves, 1); return nil }
func (s *guardStore) LoadLastState(_ context.Context) (state.LastState, error) { return state.LastState{}, state.ErrNotFound }
func (s *guardStore) Close() error { return nil }

type allowAllVerifier struct{}
func (allowAllVerifier) Verify(_ qbft.Message) error { return nil }

func TestService_WAL_Guard_DropsOlderIntents(t *testing.T) {
    dir := t.TempDir()
    // prepare wal with last intent at height=5, round=1
    w := qbft.NewWAL(filepath.Join(dir, "wal.log"))
    _ = w.AppendIntent(qbft.Message{ID:"c5", From:"n1", Type:qbft.MsgCommit, Height:5, Round:1})

    b := bus.New(4)
    s := NewWithSub(b.Subscribe())
    st := &guardStore{}
    s.SetStore(st)
    s.SetVerifier(allowAllVerifier{})
    s.SetWAL(w)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    if err := s.Start(ctx); err != nil { t.Fatalf("start: %v", err) }

    // publish an older intent (height=4) which should be dropped by guard
    b.Publish(ctx, bus.Event{Kind: bus.KindDuty, Height: 4, Round: 1})
    time.Sleep(30 * time.Millisecond)

    if atomic.LoadInt32(&st.saves) != 0 { t.Fatalf("older intent should be dropped, got saves=%d", st.saves) }
}

