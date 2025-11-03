package bls

import (
    "errors"
    bls381 "github.com/zmlAEQ/Aequa-network/internal/tss/core/bls381"
)

// 鏈€灏忓寲绫诲瀷鍗犱綅锛屽悗缁浛鎹负鐪熷疄 BLS12-381 TBLS 瀹炵幇銆?
type (
    PrivateKey         []byte                 // share private key (opaque placeholder)
    PublicKey          bls381.PubKey         // participant pubkey (compressed G1)
    GroupPublicKey     bls381.PubKey         // group pubkey (compressed G1)
    PartialSignature   bls381.Signature      // partial signature (compressed G2)
    AggregateSignature bls381.Signature      // aggregate signature (compressed G2)
)

// 鍗犱綅閿欒锛氬皻鏈疄鐜般€?
var ErrNotImplemented = errors.New("not implemented")

// PartialSign 杩斿洖鍗犱綅閿欒銆傜湡瀹炲疄鐜伴渶锛氬父鏁版椂闂淬€佸煙鍒嗙銆佹姉渚т俊閬撱€?
func PartialSign(sk PrivateKey, msg []byte, dst string) (PartialSignature, error) {
    // Real impl: evaluate signing share over msg with DST, constant-time.
    return nil, ErrNotImplemented
}

// VerifyShare 杩斿洖 false锛堝崰浣嶏級銆傜湡瀹炲疄鐜伴渶楠岃瘉鍒嗕韩绛惧悕姝ｇ‘鎬с€?
func VerifyShare(sig PartialSignature, pk PublicKey, msg []byte, dst string) bool {
    // Real impl will call bls381.Verify on partial vs participant pk
    ok, _ := bls381.Verify(bls381.PubKey(pk), bls381.Signature(sig), msg, []byte(dst))
    return ok
}

// Combine 姹囪仛浠介涓鸿仛鍚堢鍚嶏紙鍗犱綅锛夈€傜湡瀹炲疄鐜伴渶闃堝€艰仛鍚堝苟鏍￠獙浠介闆嗗悎銆?
func Combine(shares []PartialSignature) (AggregateSignature, error) {
    if len(shares) == 0 { return nil, errors.New("no shares") }
    // Real impl: Lagrange interpolation over shares -> aggregate signature
    return nil, ErrNotImplemented
}

// VerifyAgg 楠岃瘉鑱氬悎绛惧悕锛堝崰浣嶏級銆傜湡瀹炲疄鐜伴渶甯告暟鏃堕棿楠岀銆?
func VerifyAgg(sig AggregateSignature, gpk GroupPublicKey, msg []byte, dst string) bool {
    ok, _ := bls381.Verify(bls381.PubKey(gpk), bls381.Signature(sig), msg, []byte(dst))
    return ok
}

