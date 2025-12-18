//go:build blst

package pprf

import (
	"crypto/rand"
	"errors"
	"io"

	blst "github.com/supranational/blst/bindings/go"
)

var (
	ErrInvalidParams = errors.New("pprf: invalid params")
	ErrInvalidKey    = errors.New("pprf: invalid key")
	ErrPunctured     = errors.New("pprf: punctured index")
)

// LinearParams implements Construction 5.4 (Linear Setup) from
// https://eprint.iacr.org/2023/1582 (Dujmovic, Garg, Malavolta, EUROCRYPT'24).
//
// Domain indices are 1..N. Public parameters store g^{x^i} for i in
// [1..(2N+1)] excluding (N+1); both G1 and G2 are materialized to support
// asymmetric pairings (BLS12-381).
type LinearParams struct {
	N int `json:"n"`
	// G1Pows[i] = g1^{x^i} for i in [1..(2N+1)], i != N+1. Compressed (48 bytes).
	G1Pows [][]byte `json:"g1_pows"`
	// G2Pows[i] = g2^{x^i} for i in [1..(2N+1)], i != N+1. Compressed (96 bytes).
	G2Pows [][]byte `json:"g2_pows"`
}

func (p LinearParams) validate() error {
	if p.N <= 0 {
		return ErrInvalidParams
	}
	want := 2*p.N + 2
	if len(p.G1Pows) != want || len(p.G2Pows) != want {
		return ErrInvalidParams
	}
	for i := 1; i < want; i++ {
		if i == p.N+1 {
			continue
		}
		if len(p.G1Pows[i]) != 48 || len(p.G2Pows[i]) != 96 {
			return ErrInvalidParams
		}
	}
	return nil
}

// Validate checks that params match the expected shape for the linear setup.
func (p LinearParams) Validate() error { return p.validate() }

// SetupLinear generates public parameters for a bounded domain [1..n] using the
// Linear Setup construction. The parameters are public and can be shared.
func SetupLinear(n int) (LinearParams, error) {
	if n <= 0 {
		return LinearParams{}, ErrInvalidParams
	}
	x, err := randScalar()
	if err != nil {
		return LinearParams{}, err
	}
	maxPow := 2*n + 1
	g1Pows := make([][]byte, maxPow+1)
	g2Pows := make([][]byte, maxPow+1)
	pow := *x
	for i := 1; i <= maxPow; i++ {
		if i > 1 {
			if _, ok := (&pow).MulAssign(x); !ok {
				return LinearParams{}, ErrInvalidParams
			}
		}
		if i == n+1 {
			continue
		}
		g1 := blst.P1Generator().Mult(&pow)
		g1Pows[i] = g1.ToAffine().Compress()
		g2 := blst.P2Generator().Mult(&pow)
		g2Pows[i] = g2.ToAffine().Compress()
	}
	pp := LinearParams{N: n, G1Pows: g1Pows, G2Pows: g2Pows}
	if err := pp.validate(); err != nil {
		return LinearParams{}, err
	}
	return pp, nil
}

// KeyGen samples a fresh PRF key k ∈ Fr and returns it as 32-byte big-endian
// scalar encoding.
func KeyGen() ([]byte, error) {
	k, err := randScalar()
	if err != nil {
		return nil, err
	}
	return k.Serialize(), nil
}

// AddKeys returns the Fr sum of scalar keys (mod group order). Each key must be
// a 32-byte scalar encoding.
func AddKeys(keys ...[]byte) ([]byte, error) {
	if len(keys) == 0 {
		return nil, ErrInvalidKey
	}
	var acc blst.Scalar
	for i, b := range keys {
		if len(b) != 32 {
			return nil, ErrInvalidKey
		}
		var s blst.Scalar
		if s.Deserialize(b) == nil {
			return nil, ErrInvalidKey
		}
		if i == 0 {
			acc = s
			continue
		}
		if _, ok := (&acc).AddAssign(&s); !ok {
			return nil, ErrInvalidKey
		}
	}
	return acc.Serialize(), nil
}

// Eval computes PRF(pp, k, i) for i ∈ [1..N], yielding an element in GT
// serialized via Fp12 big-endian encoding.
func Eval(pp LinearParams, key []byte, i int) ([]byte, error) {
	if err := pp.validate(); err != nil {
		return nil, err
	}
	if i <= 0 || i > pp.N {
		return nil, ErrInvalidParams
	}
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	var k blst.Scalar
	if k.Deserialize(key) == nil {
		return nil, ErrInvalidKey
	}

	// Uses exponent index (N+1+i), which is never the missing (N+1) since i>=1.
	exp := pp.N + 1 + i
	var baseAff blst.P1Affine
	if baseAff.Uncompress(pp.G1Pows[exp]) == nil {
		return nil, ErrInvalidParams
	}
	var base blst.P1
	base.FromAffine(&baseAff)
	p1 := base.Mult(&k)
	p1Aff := p1.ToAffine()

	gt := blst.Fp12MillerLoop(blst.P2Generator().ToAffine(), p1Aff)
	gt.FinalExp()
	return gt.ToBendian(), nil
}

// EvalFromGK computes PRF(pp, k, i) given g1^k (compressed G1 element), without
// requiring the scalar k. This is convenient when the key is recovered via
// exponent-only threshold decryption (i.e., learning g^k but not k).
func EvalFromGK(pp LinearParams, gk []byte, i int) ([]byte, error) {
	if err := pp.validate(); err != nil {
		return nil, err
	}
	if i <= 0 || i > pp.N {
		return nil, ErrInvalidParams
	}
	if len(gk) != 48 {
		return nil, ErrInvalidKey
	}
	var gkAff blst.P1Affine
	if gkAff.Uncompress(gk) == nil {
		return nil, ErrInvalidKey
	}
	exp := pp.N + 1 + i
	var p2Aff blst.P2Affine
	if p2Aff.Uncompress(pp.G2Pows[exp]) == nil {
		return nil, ErrInvalidParams
	}
	gt := blst.Fp12MillerLoop(&p2Aff, &gkAff)
	gt.FinalExp()
	return gt.ToBendian(), nil
}

// Puncture computes k* = g1^{x^{i*}·k} for i* ∈ [1..N]. The output is a
// compressed G1 element (48 bytes).
func Puncture(pp LinearParams, key []byte, iStar int) ([]byte, error) {
	if err := pp.validate(); err != nil {
		return nil, err
	}
	if iStar <= 0 || iStar > pp.N {
		return nil, ErrInvalidParams
	}
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	var k blst.Scalar
	if k.Deserialize(key) == nil {
		return nil, ErrInvalidKey
	}
	var baseAff blst.P1Affine
	if baseAff.Uncompress(pp.G1Pows[iStar]) == nil {
		return nil, ErrInvalidParams
	}
	var base blst.P1
	base.FromAffine(&baseAff)
	out := base.Mult(&k)
	return out.ToAffine().Compress(), nil
}

// PuncturedEval computes PRF(pp, k, i) for i != iStar using the punctured key
// k* = g1^{x^{i*}·k}. When i == iStar it returns ErrPunctured.
func PuncturedEval(pp LinearParams, puncturedKey []byte, iStar, i int) ([]byte, error) {
	if err := pp.validate(); err != nil {
		return nil, err
	}
	if iStar <= 0 || iStar > pp.N || i <= 0 || i > pp.N {
		return nil, ErrInvalidParams
	}
	if i == iStar {
		return nil, ErrPunctured
	}
	if len(puncturedKey) != 48 {
		return nil, ErrInvalidKey
	}
	var kAff blst.P1Affine
	if kAff.Uncompress(puncturedKey) == nil {
		return nil, ErrInvalidKey
	}

	// Compute exponent index (N+1+i-i*). When i==i* it would be (N+1), which is
	// intentionally missing from pp; we short-circuit above.
	exp := pp.N + 1 + i - iStar
	if exp <= 0 || exp >= len(pp.G2Pows) {
		return nil, ErrInvalidParams
	}
	var p2Aff blst.P2Affine
	if p2Aff.Uncompress(pp.G2Pows[exp]) == nil {
		return nil, ErrInvalidParams
	}

	gt := blst.Fp12MillerLoop(&p2Aff, &kAff)
	gt.FinalExp()
	return gt.ToBendian(), nil
}

func randScalar() (*blst.Scalar, error) {
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ikm); err != nil {
		return nil, err
	}
	s := blst.KeyGen(ikm)
	if s == nil {
		return nil, ErrInvalidKey
	}
	return s, nil
}
