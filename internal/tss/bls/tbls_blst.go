//go:build blst

package bls

import (
    blst "github.com/supranational/blst/bindings/go"
    bls381 "github.com/zmlAEQ/Aequa-network/internal/tss/core/bls381"
)

// PartialSign signs msg under DST using a share represented by PrivateKey bytes.
// For testability we derive a blst.SecretKey from the provided bytes via KeyGen.
func PartialSign(sk PrivateKey, msg []byte, dst string) (PartialSignature, error) {
    if len(sk) == 0 { return nil, ErrNotImplemented }
    var sec blst.SecretKey
    sec.KeyGen([]byte(sk), nil)
    var sig blst.P2Affine
    sig.Sign(&sec, msg, []byte(dst), nil)
    out := sig.Serialize()
    cp := make([]byte, len(out))
    copy(cp, out)
    return PartialSignature(cp), nil
}

// VerifyShare verifies a partial signature with the participant public key.
func VerifyShare(sig PartialSignature, pk PublicKey, msg []byte, dst string) bool {
    ok, _ := bls381.Verify(bls381.PubKey(pk), bls381.Signature(sig), msg, []byte(dst))
    return ok
}

// Combine aggregates partial signatures for the same message. This uses simple
// aggregation (no Lagrange weights). Callers should verify with a list of
// pubkeys via bls381.VerifyAggregate for correctness under this combine.
func Combine(shares []PartialSignature) (AggregateSignature, error) {
    if len(shares) == 0 { return nil, ErrNotImplemented }
    agg := new(blst.P2Aggregate)
    for _, s := range shares {
        var aff blst.P2Affine
        if err := aff.Deserialize(s); err != nil { return nil, ErrNotImplemented }
        if err := agg.Aggregate(&aff, true); err != blst.SUCCESS { return nil, ErrNotImplemented }
    }
    var outAff blst.P2Affine
    outAff.FromAggregate(agg)
    out := outAff.Serialize()
    cp := make([]byte, len(out))
    copy(cp, out)
    return AggregateSignature(cp), nil
}

// VerifyAgg keeps the single-key verify path; TBLS final verify against group key
// requires Lagrange-weighted combine which is planned next. For now, prefer
// bls381.VerifyAggregate with participant pubkeys in tests.
func VerifyAgg(sig AggregateSignature, gpk GroupPublicKey, msg []byte, dst string) bool {
    ok, _ := bls381.Verify(bls381.PubKey(gpk), bls381.Signature(sig), msg, []byte(dst))
    return ok
}