//go:build blst

package bls381

import (
    blst "github.com/supranational/blst/bindings/go"
    "testing"
)

func BenchmarkSign(b *testing.B) {
    var sk blst.SecretKey; sk.KeyGen([]byte("ikm-abcdefghijklmnopqrstuvwxyz012345"), nil)
    msg := []byte("bench-msg"); dst := []byte("EQS/TSS/v1/SIG")
    for i := 0; i < b.N; i++ {
        var sig blst.P2Affine; sig.Sign(&sk, msg, dst, nil)
    }
}

func BenchmarkAggVerify(b *testing.B) {
    var sk1, sk2 blst.SecretKey
    sk1.KeyGen([]byte("ikm-1-abcdefghijklmnopqrstuvwxyz0123"), nil)
    sk2.KeyGen([]byte("ikm-2-abcdefghijklmnopqrstuvwxyz0123"), nil)
    var pk1, pk2 blst.P1Affine; pk1.From(&sk1); pk2.From(&sk2)
    msg := []byte("m"); dst := []byte("EQS/TSS/v1/SIG")
    var s1, s2 blst.P2Affine; s1.Sign(&sk1, msg, dst, nil); s2.Sign(&sk2, msg, dst, nil)
    agg, _ := Aggregate(Signature(s1.Serialize()), Signature(s2.Serialize()))
    pks := []PubKey{PubKey(pk1.Serialize()), PubKey(pk2.Serialize())}
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = VerifyAggregate(pks, agg, msg, dst)
    }
}