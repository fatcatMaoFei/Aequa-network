//go:build blst

package bte

import (
	"crypto/sha256"
	"errors"
	"sort"

	blst "github.com/supranational/blst/bindings/go"

	"github.com/zmlAEQ/Aequa-network/internal/beast/pprf"
)

var (
	ErrMissing = errors.New("bte: missing batch inputs")
)

// RecoverPRFAt derives PRF(k_i, i) from the aggregate key g^k (where
// k = Σ k_j) and a set of punctured keys {k*_j} for all j in the batch.
//
// It computes:
//
//	PRF(k_i, i) = PRF(k, i) * Π_{j≠i} PEval(k*_j, j, i)^{-1}
//
// using a multi-pairing and point negation to avoid explicit GT inversion.
// Returns a GT element encoded as Fp12 big-endian bytes.
func RecoverPRFAt(pp pprf.LinearParams, gk []byte, i int, puncturedKeys map[int][]byte) ([]byte, error) {
	if err := pp.Validate(); err != nil {
		return nil, err
	}
	if i <= 0 || i > pp.N {
		return nil, ErrInvalid
	}
	if len(gk) != 48 {
		return nil, ErrInvalid
	}
	if len(puncturedKeys) == 0 {
		return nil, ErrMissing
	}
	var gkAff blst.P1Affine
	if gkAff.Uncompress(gk) == nil {
		return nil, ErrInvalid
	}

	// numerator: PRF(k, i) = e(g^k, g^{x^{N+1+i}})
	exp0 := pp.N + 1 + i
	var q0 blst.P2Affine
	if q0.Uncompress(pp.G2Pows[exp0]) == nil {
		return nil, ErrInvalid
	}
	qs := make([]blst.P2Affine, 0, 1+len(puncturedKeys))
	ps := make([]blst.P1Affine, 0, 1+len(puncturedKeys))
	qs = append(qs, q0)
	ps = append(ps, gkAff)

	// denominator contributions in deterministic order
	other := make([]int, 0, len(puncturedKeys))
	for j := range puncturedKeys {
		if j == i {
			continue
		}
		other = append(other, j)
	}
	sort.Ints(other)
	for _, j := range other {
		kStar := puncturedKeys[j]
		if len(kStar) != 48 {
			return nil, ErrInvalid
		}
		exp := pp.N + 1 + i - j
		var q blst.P2Affine
		if q.Uncompress(pp.G2Pows[exp]) == nil {
			return nil, ErrInvalid
		}
		var pAff blst.P1Affine
		if pAff.Uncompress(kStar) == nil {
			return nil, ErrInvalid
		}
		qs = append(qs, q)
		ps = append(ps, negP1Affine(&pAff))
	}
	gt := blst.Fp12MillerLoopN(qs, ps)
	gt.FinalExp()
	return gt.ToBendian(), nil
}

// RecoverXOR unmasks beta by XOR'ing it with SHA256(prfGT).
func RecoverXOR(beta []byte, prfGT []byte) []byte {
	if len(beta) == 0 {
		return nil
	}
	sum := sha256.Sum256(prfGT)
	out := make([]byte, len(beta))
	for i := range beta {
		out[i] = beta[i] ^ sum[i%len(sum)]
	}
	return out
}

func negP1Affine(a *blst.P1Affine) blst.P1Affine {
	var p blst.P1
	p.FromAffine(a)
	var neg blst.P1
	neg.SubAssign(&p)
	return *neg.ToAffine()
}
