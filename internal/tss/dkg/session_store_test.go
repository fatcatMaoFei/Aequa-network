package dkg

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestSessionStore_SaveLoad_OK(t *testing.T) {
    dir := t.TempDir(); ss := NewSessionStore(dir)
    st := sessionState{Epoch: 2, Propose: []string{"P1"}, Commit: []string{"P2"}, Reveal: []string{"P3"}, Ack: []string{"P1","P2"}, Done: false}
    if err := ss.Save("sid", st); err != nil { t.Fatalf("save: %v", err) }
    got, err := ss.Load("sid"); if err != nil { t.Fatalf("load: %v", err) }
    if got.Epoch != 2 || len(got.Ack) != 2 { t.Fatalf("mismatch: %+v", got) }
    if _, err := readSess(filepath.Join(dir, "tss_session_sid.dat")); err != nil { t.Fatalf("read file: %v", err) }
}

func TestSessionStore_Load_FallbackFromBak(t *testing.T) {
    dir := t.TempDir(); ss := NewSessionStore(dir)
    // v1
    if err := ss.Save("sid2", sessionState{Epoch:1}); err != nil { t.Fatalf("save v1: %v", err) }
    // v2 to create .bak of v1
    if err := ss.Save("sid2", sessionState{Epoch:2}); err != nil { t.Fatalf("save v2: %v", err) }
    // Corrupt main file
    p := filepath.Join(dir, "tss_session_sid2.dat")
    if err := os.Truncate(p, 8); err != nil { t.Fatalf("truncate: %v", err) }
    // Load should fallback to .bak (epoch=1)
    got, err := ss.Load("sid2"); if err != nil { t.Fatalf("load: %v", err) }
    if got.Epoch != 1 { t.Fatalf("fallback epoch mismatch: %+v", got) }
    _ = context.Background() // keep context import used to avoid lint noise
}