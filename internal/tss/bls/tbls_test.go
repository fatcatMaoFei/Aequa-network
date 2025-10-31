package bls

import "testing"

func TestTBLS_Stubs(t *testing.T) {
    if _, err := PartialSign(nil, []byte("m"), "EQS/TSS/v1/SIG"); err == nil {
        t.Fatalf("want not implemented")
    }
    if VerifyShare(nil, nil, nil, "EQS/TSS/v1/SIG") {
        t.Fatalf("verify share should be false (stub)")
    }
    if _, err := Combine(nil); err == nil {
        t.Fatalf("want error on empty shares")
    }
    if VerifyAgg(nil, nil, nil, "EQS/TSS/v1/SIG") {
        t.Fatalf("verify agg should be false (stub)")
    }
}

