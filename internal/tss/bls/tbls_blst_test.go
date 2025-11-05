//go:build blst

package bls

import (
    blst "github.com/supranational/blst/bindings/go"
    bls381 "github.com/zmlAEQ/Aequa-network/internal/tss/core/bls381"
    "testing"
)

func TestTBLS_blst_SignCombineVerifyAggregate(t *testing.T) {
    var sk1, sk2 blst.SecretKey
    sk1.KeyGen([]byte("ikm-1-abcdefghijklmnopqrstuvwxyz0123"), nil)
    sk2.KeyGen([]byte("ikm-2-abcdefghijklmnopqrstuvwxyz0123"), nil)
    var pk1, pk2 blst.P1Affine
    pk1.From(&sk1)
    pk2.From(&sk2)

    msg := []byte("m")
    dst := "EQS/TSS/v1/SIG"

    s1, err := PartialSign(PrivateKey([]byte("ikm-1-abcdefghijklmnopqrstuvwxyz0123")), msg, dst)
    if err != nil { t.Fatalf("sign1: %v", err) }
    s2, err := PartialSign(PrivateKey([]byte("ikm-2-abcdefghijklmnopqrstuvwxyz0123")), msg, dst)
    if err != nil { t.Fatalf("sign2: %v", err) }

    if !VerifyShare(s1, PublicKey(pk1.Serialize()), msg, dst) { t.Fatalf("share1 verify") }
    if !VerifyShare(s2, PublicKey(pk2.Serialize()), msg, dst) { t.Fatalf("share2 verify") }

    agg, err := Combine([]PartialSignature{s1, s2})
    if err != nil { t.Fatalf("combine: %v", err) }
    ok, err := bls381.VerifyAggregate([]bls381.PubKey{bls381.PubKey(pk1.Serialize()), bls381.PubKey(pk2.Serialize())}, bls381.Signature(agg), msg, []byte(dst))
    if err != nil || !ok { t.Fatalf("agg verify err=%v ok=%v", err, ok) }
}