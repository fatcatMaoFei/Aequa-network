package dkg

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func sampleShare() KeyShare {
    return KeyShare{Index: 1, PublicKey: []byte{1,2,3}, PrivateKey: []byte{4,5,6}, Commitments: [][]byte{{7,8}}}
}

func TestKeyStore_SaveLoad_OK(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "tss_keyshare.dat")
    s := NewKeyStore(path)
    want := sampleShare()
    if err := s.SaveKeyShare(context.Background(), want); err != nil {
        t.Fatalf("save: %v", err)
    }
    got, err := s.LoadKeyShare(context.Background())
    if err != nil { t.Fatalf("load: %v", err) }
    if len(got.PublicKey) != len(want.PublicKey) || len(got.PrivateKey) != len(want.PrivateKey) || len(got.Commitments) != len(want.Commitments) || got.Index != want.Index {
        t.Fatalf("mismatch: got=%+v want=%+v", got, want)
    }
}

func TestKeyStore_Load_Fallback_OnCorruption(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "tss_keyshare.dat")
    s := NewKeyStore(path)
    // Save v1
    if err := s.SaveKeyShare(context.Background(), KeyShare{Index: 1, PublicKey: []byte{1}}); err != nil {
        t.Fatalf("save1: %v", err)
    }
    // Save v2 (creates .bak of v1)
    if err := s.SaveKeyShare(context.Background(), KeyShare{Index: 2, PublicKey: []byte{2}}); err != nil {
        t.Fatalf("save2: %v", err)
    }
    // Corrupt main
    if err := os.Truncate(path, 8); err != nil {
        t.Fatalf("truncate: %v", err)
    }
    got, err := s.LoadKeyShare(context.Background())
    if err != nil { t.Fatalf("load after corrupt: %v", err) }
    if got.Index != 1 { t.Fatalf("fallback mismatch: got=%+v want Index=1", got) }
}

func TestKeyStore_NotFound(t *testing.T) {
    dir := t.TempDir()
    s := NewKeyStore(filepath.Join(dir, "missing.dat"))
    if _, err := s.LoadKeyShare(context.Background()); err == nil {
        t.Fatalf("want ErrNotFound")
    }
}

