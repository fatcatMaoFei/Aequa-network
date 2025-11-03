package bls

import "testing"

// FuzzTBLS_NoPanic ensures current stubs never panic and handle shapes.
func FuzzTBLS_NoPanic(f *testing.F) {
    f.Add(uint8(0), uint8(0))
    f.Fuzz(func(t *testing.T, a, b uint8) {
        _, _ = PartialSign(nil, []byte{a, b}, "EQS/TSS/v1/SIG")
        _ = VerifyShare(nil, nil, []byte{a}, "EQS/TSS/v1/SIG")
        _, _ = Combine([]PartialSignature{{}, {}})
        _ = VerifyAgg(nil, nil, []byte{b}, "EQS/TSS/v1/SIG")
    })
}