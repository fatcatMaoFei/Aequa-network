//go:build blst

package bls381

import (
    blst "github.com/supranational/blst/bindings/go"
)

// HashToG2 maps msg to a compressed G2 point using the provided DST.
func HashToG2(msg, dst []byte) (G2Point, error) {
    p := new(blst.P2)
    p.HashTo(msg, dst, nil)
    var aff blst.P2Affine
    aff.From(p)
    out := aff.Serialize()
    // Serialize returns 96‑byte compressed form
    cp := make([]byte, len(out))
    copy(cp, out)
    return G2Point(cp), nil
}

// Verify checks a BLS signature against a pubkey and message under DST.
// pk is compressed G1 (48 bytes); sig is compressed G2 (96 bytes).
func Verify(pk PubKey, sig Signature, msg, dst []byte) (bool, error) {
    if len(pk) == 0 || len(sig) == 0 { return false, ErrInvalidInput }
    var pkAff blst.P1Affine
    if err := pkAff.Deserialize(pk); err != nil { return false, ErrInvalidInput }
    var sigAff blst.P2Affine
    if err := sigAff.Deserialize(sig); err != nil { return false, ErrInvalidInput }
    // core verify (hash‑to‑curve inside blst)
    ok := sigAff.Verify(true, &pkAff, true, msg, dst, nil) == blst.SUCCESS
    return ok, nil
}

// Aggregate combines multiple signatures (compressed G2) into one.
func Aggregate(sigs ...Signature) (Signature, error) {
    if len(sigs) == 0 { return nil, ErrInvalidInput }
    agg := new(blst.P2Aggregate)
    for _, s := range sigs {
        var aff blst.P2Affine
        if err := aff.Deserialize(s); err != nil { return nil, ErrInvalidInput }
        if err := agg.Aggregate(&aff, true); err != blst.SUCCESS { return nil, ErrInvalidInput }
    }
    var outAff blst.P2Affine
    outAff.FromAggregate(agg)
    out := outAff.Serialize()
    cp := make([]byte, len(out))
    copy(cp, out)
    return Signature(cp), nil
}

// VerifyAggregate verifies an aggregate signature for the same message.
func VerifyAggregate(pks []PubKey, sig Signature, msg, dst []byte) (bool, error) {
    if len(pks) == 0 || len(sig) == 0 { return false, ErrInvalidInput }
    var sigAff blst.P2Affine
    if err := sigAff.Deserialize(sig); err != nil { return false, ErrInvalidInput }
    arr := make([]*blst.P1Affine, 0, len(pks))
    for _, pk := range pks {
        var a blst.P1Affine
        if err := a.Deserialize(pk); err != nil { return false, ErrInvalidInput }
        arr = append(arr, &a)
    }
    ok := blst.FastAggregateVerify(&sigAff, arr, msg, dst) == blst.SUCCESS
    return ok, nil
}