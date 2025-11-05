package dkg

import (
    "path/filepath"
    "testing"
)

func TestSessionStore_SaveLoad_OK(t *testing.T) {
    dir := t.TempDir()
    ss := NewSessionStore(dir)
    st := sessionState{Epoch: 2, Propose: []string{"P1"}, Commit: []string{"P2"}, Reveal: []string{"P3"}, Ack: []string{"P1","P2"}, Done: false}
    if err := ss.Save("sid", st); err != nil { t.Fatalf("save: %v", err) }
    got, err := ss.Load("sid")
    if err != nil { t.Fatalf("load: %v", err) }
    if got.Epoch != 2 || len(got.Ack) != 2 { t.Fatalf("mismatch: %+v", got) }
    if _, err := readSess(filepath.Join(dir, "tss_session_sid.dat")); err != nil { t.Fatalf("read file: %v", err) }
}