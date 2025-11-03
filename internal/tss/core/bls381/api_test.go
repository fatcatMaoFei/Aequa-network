package bls381

import "testing"

func TestStubs_NotImplemented(t *testing.T) {
    if _, err := HashToG2([]byte("m"), []byte("DST")); err == nil {
        t.Fatalf("want not implemented")
    }
    if ok, err := Verify(nil, nil, nil, nil); err == nil || ok {
        t.Fatalf("verify should be stub")
    }
    if _, err := Aggregate(nil); err == nil {
        t.Fatalf("aggregate should be stub")
    }
    if ok, err := VerifyAggregate(nil, nil, nil, nil); err == nil || ok {
        t.Fatalf("verify agg should be stub")
    }
}

