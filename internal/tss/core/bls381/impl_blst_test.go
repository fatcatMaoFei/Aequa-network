//go:build blst

package bls381

import (
    blst "github.com/supranational/blst/bindings/go"
    "testing"
)

// Note: These tests compile/run only with -tags blst; default CI doesn't trigger them.
func TestVerify_SignRoundtrip(t *testing.T) {
    // Generate a keypair
    ikm := []byte("ikm-32-bytes-minimum-length-012345")
    var sk blst.SecretKey
    sk.KeyGen(ikm, nil)

    var pkAff blst.P1Affine
    pkAff.From(&sk)
    pk := pkAff.Serialize()

    msg := []byte("hello")
    dst := []byte("EQS/TSS/v1/SIG")

    // Sign
    var sigAff blst.P2Affine
    sigAff.Sign(&sk, msg, dst, nil)
    sig := sigAff.Serialize()

    ok, err := Verify(PubKey(pk), Signature(sig), msg, dst)
    if err != nil || !ok { t.Fatalf("verify err=%v ok=%v", err, ok) }
}

func TestAggregate_FastVerify(t *testing.T) {
    ikm1 := []byte("ikm-1-abcdefghijklmnopqrstuvwxyz0123")
    ikm2 := []byte("ikm-2-abcdefghijklmnopqrstuvwxyz0123")
    var sk1, sk2 blst.SecretKey
    sk1.KeyGen(ikm1, nil)
    sk2.KeyGen(ikm2, nil)

    msg := []byte("m")
    dst := []byte("EQS/TSS/v1/SIG")

    var pk1, pk2 blst.P1Affine
    pk1.From(&sk1); pk2.From(&sk2)
    var sig1, sig2 blst.P2Affine
    sig1.Sign(&sk1, msg, dst, nil)
    sig2.Sign(&sk2, msg, dst, nil)

    agg, err := Aggregate(Signature(sig1.Serialize()), Signature(sig2.Serialize()))
    if err != nil { t.Fatalf("agg: %v", err) }
    ok, err := VerifyAggregate([]PubKey{PubKey(pk1.Serialize()), PubKey(pk2.Serialize())}, agg, msg, dst)
    if err != nil || !ok { t.Fatalf("verify agg err=%v ok=%v", err, ok) }
}