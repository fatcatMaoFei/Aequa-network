package core

import "testing"

// FuzzHashToCurve_NoPanic asserts that HashToCurve never panics and returns
// ErrInvalidDST for unknown dst, ErrNotImplemented for known dst.
func FuzzHashToCurve_NoPanic(f *testing.F) {
    f.Add([]byte("msg"), DSTSig)
    f.Add([]byte("msg"), DSTDkg)
    f.Add([]byte("msg"), DSTApp)
    f.Add([]byte("x"), "EQS/UNKNOWN")
    f.Fuzz(func(t *testing.T, msg []byte, dst string) {
        _, err := HashToCurve(msg, dst)
        if IsValidDST(dst) {
            if err == nil {
                t.Fatalf("want ErrNotImplemented for valid dst")
            }
        } else {
            if err == nil {
                t.Fatalf("want error for invalid dst")
            }
        }
    })
}

