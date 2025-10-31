package core

import "testing"

func TestIsValidDST(t *testing.T) {
    if !IsValidDST(DSTSig) || !IsValidDST(DSTDkg) || !IsValidDST(DSTApp) {
        t.Fatalf("known DST should be valid")
    }
    if IsValidDST("EQS/UNKNOWN") {
        t.Fatalf("unexpected valid DST")
    }
}

func TestHashToCurve_Stub_NotImplemented(t *testing.T) {
    if _, err := HashToCurve([]byte("msg"), DSTSig); err == nil {
        t.Fatalf("want not implemented")
    }
}

func TestHashToCurve_InvalidDST(t *testing.T) {
    if _, err := HashToCurve([]byte("msg"), "X/BAD"); err == nil {
        t.Fatalf("want invalid dst error")
    }
}

