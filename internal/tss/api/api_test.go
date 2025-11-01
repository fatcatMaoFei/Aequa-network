package api

import (
    "context"
    "testing"
)

func TestAPI_Stubs(t *testing.T) {
    s := New()
    if _, err := s.Sign(context.Background(), Duty{Height: 1, Round: 0}, []byte("m")); err == nil {
        t.Fatalf("want not implemented")
    }
    if s.VerifyAgg(nil, nil, nil) {
        t.Fatalf("verify should be false (stub)")
    }
    if err := s.Resume("sess"); err == nil {
        t.Fatalf("want not implemented for resume")
    }
}

