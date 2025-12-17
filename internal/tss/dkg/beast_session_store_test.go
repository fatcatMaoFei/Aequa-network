package dkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBeastSessionStore_SaveLoad_OK(t *testing.T) {
	dir := t.TempDir()
	s := NewBeastSessionStore(dir)
	want := beastSessionState{
		Epoch: 2,
		Coeffs: [][]byte{
			{1, 2, 3},
		},
		SelfCommitments: [][]byte{
			make([]byte, 48),
		},
		Commitments: map[int][][]byte{
			1: {make([]byte, 48)},
		},
		Shares: map[int][]byte{
			1: make([]byte, 32),
		},
		Done:        true,
		GroupPubKey: make([]byte, 48),
		ShareScalar: make([]byte, 32),
	}
	if err := s.Save("sess", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load("sess")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Epoch != want.Epoch || got.Done != want.Done || len(got.Coeffs) != len(want.Coeffs) || len(got.Commitments) != len(want.Commitments) || len(got.Shares) != len(want.Shares) {
		t.Fatalf("mismatch: got=%+v want=%+v", got, want)
	}
}

func TestBeastSessionStore_Load_Fallback_OnCorruption(t *testing.T) {
	dir := t.TempDir()
	s := NewBeastSessionStore(dir)

	st1 := beastSessionState{Epoch: 1, Coeffs: [][]byte{{1}}}
	st2 := beastSessionState{Epoch: 2, Coeffs: [][]byte{{2}}}
	if err := s.Save("sess", st1); err != nil {
		t.Fatalf("save1: %v", err)
	}
	if err := s.Save("sess", st2); err != nil {
		t.Fatalf("save2: %v", err)
	}

	// Corrupt main file; expect fallback to .bak (st1).
	mainPath := filepath.Join(dir, "beast_dkg_session_sess.dat")
	if err := os.Truncate(mainPath, 8); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	got, err := s.Load("sess")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Epoch != 1 {
		t.Fatalf("unexpected epoch: got=%d want=1", got.Epoch)
	}
}

func TestBeastSessionStore_NotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewBeastSessionStore(dir)
	if _, err := s.Load("missing"); err == nil {
		t.Fatalf("expected ErrBeastSessionNotFound")
	}
}

