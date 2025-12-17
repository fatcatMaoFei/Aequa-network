//go:build blst

package dkg

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	blst "github.com/supranational/blst/bindings/go"
)

var (
	ErrInvalidParams = errors.New("invalid params")
	ErrInvalidPoint  = errors.New("invalid point")
	ErrInvalidShare  = errors.New("invalid share")
)

type scalarShare struct {
	Index int
	Value *blst.Scalar
}

func randScalar(r io.Reader) (*blst.Scalar, error) {
	var ikm [32]byte
	if _, err := io.ReadFull(r, ikm[:]); err != nil {
		return nil, err
	}
	sk := blst.KeyGen(ikm[:], nil)
	if sk == nil {
		return nil, errors.New("bad randomness")
	}
	return sk, nil
}

func scalarFromInt(v int) *blst.Scalar {
	var buf [blst.BLST_SCALAR_BYTES]byte
	binary.BigEndian.PutUint64(buf[len(buf)-8:], uint64(v))
	var s blst.Scalar
	_ = s.FromBEndian(buf[:])
	return &s
}

func evalPolyAt(coeffs []*blst.Scalar, x int) (*blst.Scalar, error) {
	if len(coeffs) == 0 || x <= 0 {
		return nil, ErrInvalidParams
	}
	xs := scalarFromInt(x)
	acc := scalarFromInt(0)
	pow := scalarFromInt(1)
	for _, c := range coeffs {
		if c == nil {
			return nil, ErrInvalidParams
		}
		term, ok := c.Mul(pow)
		if !ok {
			return nil, ErrInvalidShare
		}
		if _, ok := acc.AddAssign(term); !ok {
			return nil, ErrInvalidShare
		}
		nxt, ok := pow.Mul(xs)
		if !ok {
			return nil, ErrInvalidShare
		}
		pow = nxt
	}
	return acc, nil
}

// commitmentsFromPoly returns Feldman commitments C_j = g1^{a_j} as compressed G1 points.
func commitmentsFromPoly(coeffs []*blst.Scalar) ([][]byte, error) {
	if len(coeffs) == 0 {
		return nil, ErrInvalidParams
	}
	out := make([][]byte, 0, len(coeffs))
	for _, c := range coeffs {
		if c == nil {
			return nil, ErrInvalidParams
		}
		p := blst.P1Generator().Mult(c)
		out = append(out, p.ToAffine().Compress())
	}
	return out, nil
}

func verifyFeldmanShare(share *blst.Scalar, x int, commitments [][]byte) (bool, error) {
	if share == nil || x <= 0 || len(commitments) == 0 {
		return false, ErrInvalidParams
	}
	// lhs = g1^{share}
	lhs := blst.P1Generator().Mult(share).ToAffine().Compress()
	// rhs = Σ C_j * x^j
	xs := scalarFromInt(x)
	pow := scalarFromInt(1)
	acc := new(blst.P1)
	for _, cBytes := range commitments {
		var aff blst.P1Affine
		if aff.Uncompress(cBytes) == nil {
			return false, ErrInvalidPoint
		}
		var p blst.P1
		p.FromAffine(&aff)
		p.MultAssign(pow)
		acc.AddAssign(&p)
		nxt, ok := pow.Mul(xs)
		if !ok {
			return false, ErrInvalidShare
		}
		pow = nxt
	}
	rhs := acc.ToAffine().Compress()
	if len(lhs) != len(rhs) {
		return false, nil
	}
	for i := range lhs {
		if lhs[i] != rhs[i] {
			return false, nil
		}
	}
	return true, nil
}

// lagrangeAtZeroScalar computes λ_i(0) for Shamir shares with indices in indices.
func lagrangeAtZeroScalar(i int, indices []int) (*blst.Scalar, error) {
	if i <= 0 || len(indices) == 0 {
		return nil, ErrInvalidParams
	}
	xi := scalarFromInt(i)
	num := scalarFromInt(1)
	den := scalarFromInt(1)
	zero := scalarFromInt(0)
	for _, j := range indices {
		if j == i {
			continue
		}
		if j <= 0 {
			return nil, ErrInvalidParams
		}
		xj := scalarFromInt(j)
		neg, ok := zero.Sub(xj)
		if !ok {
			return nil, ErrInvalidShare
		}
		num, ok = num.Mul(neg)
		if !ok {
			return nil, ErrInvalidShare
		}
		diff, ok := xi.Sub(xj)
		if !ok {
			return nil, ErrInvalidShare
		}
		den, ok = den.Mul(diff)
		if !ok {
			return nil, ErrInvalidShare
		}
	}
	inv := den.Inverse()
	out, ok := num.Mul(inv)
	if !ok {
		return nil, ErrInvalidShare
	}
	return out, nil
}

func combineScalarSharesAtZero(shares []scalarShare, k int) (*blst.Scalar, error) {
	if k <= 0 || len(shares) < k {
		return nil, ErrInvalidParams
	}
	sort.Slice(shares, func(i, j int) bool { return shares[i].Index < shares[j].Index })
	shares = shares[:k]
	indices := make([]int, 0, len(shares))
	seen := map[int]struct{}{}
	for _, s := range shares {
		if s.Index <= 0 || s.Value == nil {
			return nil, ErrInvalidParams
		}
		if _, ok := seen[s.Index]; ok {
			return nil, ErrInvalidParams
		}
		seen[s.Index] = struct{}{}
		indices = append(indices, s.Index)
	}
	acc := scalarFromInt(0)
	for _, s := range shares {
		coeff, err := lagrangeAtZeroScalar(s.Index, indices)
		if err != nil {
			return nil, err
		}
		term, ok := s.Value.Mul(coeff)
		if !ok {
			return nil, ErrInvalidShare
		}
		if _, ok := acc.AddAssign(term); !ok {
			return nil, ErrInvalidShare
		}
	}
	return acc, nil
}

// SimulateFeldmanDKG runs a minimal Feldman DKG in-memory (no network) and returns
// group public key and per-participant secret shares (scalar bytes).
// This is a helper for tests and for wiring DKG into higher layers.
func SimulateFeldmanDKG(n int, t int) (groupPubKey []byte, shares map[int][]byte, err error) {
	if n <= 0 || t <= 0 || t > n {
		return nil, nil, ErrInvalidParams
	}
	type dealer struct {
		coeffs      []*blst.Scalar
		commitments [][]byte
	}
	dealers := make(map[int]dealer, n)
	for idx := 1; idx <= n; idx++ {
		coeffs := make([]*blst.Scalar, 0, t)
		for j := 0; j < t; j++ {
			s, e := randScalar(rand.Reader)
			if e != nil {
				return nil, nil, e
			}
			coeffs = append(coeffs, s)
		}
		com, e := commitmentsFromPoly(coeffs)
		if e != nil {
			return nil, nil, e
		}
		dealers[idx] = dealer{coeffs: coeffs, commitments: com}
	}

	// group pk = Σ dealer.commitments[0]
	acc := new(blst.P1)
	for idx := 1; idx <= n; idx++ {
		c0 := dealers[idx].commitments[0]
		var aff blst.P1Affine
		if aff.Uncompress(c0) == nil {
			return nil, nil, ErrInvalidPoint
		}
		var p blst.P1
		p.FromAffine(&aff)
		acc.AddAssign(&p)
	}
	groupPubKey = acc.ToAffine().Compress()

	// each participant i receives share from every dealer, verifies, sums -> final share
	shares = make(map[int][]byte, n)
	for i := 1; i <= n; i++ {
		sum := scalarFromInt(0)
		for dealerIdx := 1; dealerIdx <= n; dealerIdx++ {
			d := dealers[dealerIdx]
			si, e := evalPolyAt(d.coeffs, i)
			if e != nil {
				return nil, nil, e
			}
			ok, e := verifyFeldmanShare(si, i, d.commitments)
			if e != nil {
				return nil, nil, e
			}
			if !ok {
				return nil, nil, ErrInvalidShare
			}
			if _, ok := sum.AddAssign(si); !ok {
				return nil, nil, ErrInvalidShare
			}
		}
		shares[i] = sum.Serialize()
	}
	return groupPubKey, shares, nil
}

