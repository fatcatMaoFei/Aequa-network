package qbft

import (
    "os"
    "path/filepath"
    "testing"
)

func TestWAL_AppendAndLast(t *testing.T) {
    dir := t.TempDir()
    w := NewWAL(filepath.Join(dir, "wal.log"))
    // append prepare then commit
    if err := w.AppendIntent(Message{ID:"p1", From:"n1", Type:MsgPrepare, Height:1, Round:1}); err != nil { t.Fatalf("append1: %v", err) }
    if err := w.AppendIntent(Message{ID:"c1", From:"n2", Type:MsgCommit, Height:1, Round:1}); err != nil { t.Fatalf("append2: %v", err) }
    last, err := w.LastIntent()
    if err != nil { t.Fatalf("last: %v", err) }
    if last.Type != MsgCommit || last.ID != "c1" { t.Fatalf("want commit c1, got %+v", last) }
}

func TestWAL_Last_NoFile(t *testing.T) {
    dir := t.TempDir()
    w := NewWAL(filepath.Join(dir, "missing.log"))
    if _, err := w.LastIntent(); err == nil {
        t.Fatalf("want error on missing wal")
    }
}

func TestWAL_IgnoreNonVoteTypes(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "wal.log")
    w := NewWAL(p)
    // preprepare should be ignored without error
    if err := w.AppendIntent(Message{ID:"pp", From:"l", Type:MsgPreprepare}); err != nil { t.Fatalf("append: %v", err) }
    if _, err := os.Stat(p); err == nil {
        t.Fatalf("file should not be created for non-vote intents")
    }
}

