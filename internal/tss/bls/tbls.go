package bls

import (
    "errors"
    bls381 "github.com/zmlAEQ/Aequa-network/internal/tss/core/bls381"
)

// 最小化类型占位，后续替换为真实 BLS12-381 TBLS 实现。
type (
    PrivateKey         []byte                 // share private key (opaque placeholder)
    PublicKey          bls381.PubKey         // participant pubkey (compressed G1)
    GroupPublicKey     bls381.PubKey         // group pubkey (compressed G1)
    PartialSignature   bls381.Signature      // partial signature (compressed G2)
    AggregateSignature bls381.Signature      // aggregate signature (compressed G2)
)

// 占位错误：尚未实现。
var ErrNotImplemented = errors.New("not implemented")

// PartialSign 返回占位错误。真实实现需：常数时间、域分离、抗侧信道。
func PartialSign(sk PrivateKey, msg []byte, dst string) (PartialSignature, error) {
    // Real impl: evaluate signing share over msg with DST, constant-time.
    return nil, ErrNotImplemented
}

// VerifyShare 返回 false（占位）。真实实现需验证分享签名正确性。
func VerifyShare(sig PartialSignature, pk PublicKey, msg []byte, dst string) bool {
    // Real impl will call bls381.Verify on partial vs participant pk
    ok, _ := bls381.Verify(pk, bls381.Signature(sig), msg, []byte(dst))
    return ok
}

// Combine 汇聚份额为聚合签名（占位）。真实实现需阈值聚合并校验份额集合。
func Combine(shares []PartialSignature) (AggregateSignature, error) {
    if len(shares) == 0 { return nil, errors.New("no shares") }
    // Real impl: Lagrange interpolation over shares -> aggregate signature
    return nil, ErrNotImplemented
}

// VerifyAgg 验证聚合签名（占位）。真实实现需常数时间验签。
func VerifyAgg(sig AggregateSignature, gpk GroupPublicKey, msg []byte, dst string) bool {
    ok, _ := bls381.Verify(bls381.PubKey(gpk), bls381.Signature(sig), msg, []byte(dst))
    return ok
}

