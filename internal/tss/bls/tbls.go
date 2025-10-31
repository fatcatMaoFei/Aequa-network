package bls

import "errors"

// 最小化类型占位，后续替换为真实 BLS12-381 TBLS 实现。
type (
    PrivateKey        []byte
    PublicKey         []byte
    GroupPublicKey    []byte
    PartialSignature  []byte
    AggregateSignature []byte
)

// 占位错误：尚未实现。
var ErrNotImplemented = errors.New("not implemented")

// PartialSign 返回占位错误。真实实现需：常数时间、域分离、抗侧信道。
func PartialSign(sk PrivateKey, msg []byte, dst string) (PartialSignature, error) {
    return nil, ErrNotImplemented
}

// VerifyShare 返回 false（占位）。真实实现需验证分享签名正确性。
func VerifyShare(sig PartialSignature, pk PublicKey, msg []byte, dst string) bool {
    return false
}

// Combine 汇聚份额为聚合签名（占位）。真实实现需阈值聚合并校验份额集合。
func Combine(shares []PartialSignature) (AggregateSignature, error) {
    if len(shares) == 0 { return nil, errors.New("no shares") }
    return nil, ErrNotImplemented
}

// VerifyAgg 验证聚合签名（占位）。真实实现需常数时间验签。
func VerifyAgg(sig AggregateSignature, gpk GroupPublicKey, msg []byte, dst string) bool {
    return false
}

