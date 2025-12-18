//go:build blst

package bte

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	blst "github.com/supranational/blst/bindings/go"
)

var (
	ErrInvalid = errors.New("bte: invalid input")
)

// KeyCiphertext is an ElGamal-in-the-exponent ciphertext over G1:
//
//	C1 = g^r
//	C2 = pk^r * g^k
//
// where pk = g^s is the threshold public key and k ∈ Fr is the plaintext scalar
// encoded as g^k.
type KeyCiphertext struct {
	C1 []byte `json:"c1"` // compressed G1 (48 bytes)
	C2 []byte `json:"c2"` // compressed G1 (48 bytes)
}

type PartialDecryptShare struct {
	Index int    `json:"index"`
	Share []byte `json:"share"` // compressed G1 (48 bytes): C1^{s_i}
}

// EncryptKey encrypts a key scalar k under a public key pk (compressed G1).
// It returns a ciphertext that can be homomorphically aggregated by group
// addition (i.e., multiplying ciphertexts in multiplicative notation).
func EncryptKey(pk []byte, keyScalar []byte) (KeyCiphertext, error) {
	if len(pk) != 48 || len(keyScalar) != 32 {
		return KeyCiphertext{}, ErrInvalid
	}
	var pkAff blst.P1Affine
	if pkAff.Uncompress(pk) == nil {
		return KeyCiphertext{}, ErrInvalid
	}
	var k blst.Scalar
	if k.Deserialize(keyScalar) == nil {
		return KeyCiphertext{}, ErrInvalid
	}
	r, err := randScalar()
	if err != nil {
		return KeyCiphertext{}, err
	}
	// m = g^k
	m := blst.P1Generator().Mult(&k)

	// C1 = g^r
	c1 := blst.P1Generator().Mult(r)

	// pk^r
	var pkP1 blst.P1
	pkP1.FromAffine(&pkAff)
	pkR := pkP1.Mult(r)
	// C2 = pk^r + m
	pkR.AddAssign(m)

	return KeyCiphertext{
		C1: c1.ToAffine().Compress(),
		C2: pkR.ToAffine().Compress(),
	}, nil
}

// AddCiphertexts homomorphically aggregates ciphertexts by component-wise
// group addition in G1.
func AddCiphertexts(cts []KeyCiphertext) (KeyCiphertext, error) {
	if len(cts) == 0 {
		return KeyCiphertext{}, ErrInvalid
	}
	var acc1 blst.P1
	var acc2 blst.P1
	for _, ct := range cts {
		if len(ct.C1) != 48 || len(ct.C2) != 48 {
			return KeyCiphertext{}, ErrInvalid
		}
		var c1Aff blst.P1Affine
		if c1Aff.Uncompress(ct.C1) == nil {
			return KeyCiphertext{}, ErrInvalid
		}
		var c1 blst.P1
		c1.FromAffine(&c1Aff)
		acc1.AddAssign(&c1)

		var c2Aff blst.P1Affine
		if c2Aff.Uncompress(ct.C2) == nil {
			return KeyCiphertext{}, ErrInvalid
		}
		var c2 blst.P1
		c2.FromAffine(&c2Aff)
		acc2.AddAssign(&c2)
	}
	return KeyCiphertext{
		C1: acc1.ToAffine().Compress(),
		C2: acc2.ToAffine().Compress(),
	}, nil
}

// PartialDecrypt computes a threshold decryption share for a ciphertext using a
// Shamir secret share s_i (32-byte scalar encoding).
func PartialDecrypt(ct KeyCiphertext, shareScalar []byte, index int) (PartialDecryptShare, error) {
	if index <= 0 {
		return PartialDecryptShare{}, ErrInvalid
	}
	if len(ct.C1) != 48 {
		return PartialDecryptShare{}, ErrInvalid
	}
	if len(shareScalar) != 32 {
		return PartialDecryptShare{}, ErrInvalid
	}
	var s blst.Scalar
	if s.Deserialize(shareScalar) == nil {
		return PartialDecryptShare{}, ErrInvalid
	}
	var c1Aff blst.P1Affine
	if c1Aff.Uncompress(ct.C1) == nil {
		return PartialDecryptShare{}, ErrInvalid
	}
	var c1 blst.P1
	c1.FromAffine(&c1Aff)
	out := c1.Mult(&s)
	return PartialDecryptShare{Index: index, Share: out.ToAffine().Compress()}, nil
}

// DecryptKeyG recovers g^k from an aggregated ciphertext using >=threshold
// partial decrypt shares (Shamir at x=0). It returns the message as a
// compressed G1 element (48 bytes).
func DecryptKeyG(ct KeyCiphertext, shares []PartialDecryptShare, threshold int) ([]byte, error) {
	if threshold <= 0 || len(shares) < threshold {
		return nil, ErrInvalid
	}
	if len(ct.C2) != 48 {
		return nil, ErrInvalid
	}
	// Deterministic subset: smallest threshold indices.
	sort.Slice(shares, func(i, j int) bool { return shares[i].Index < shares[j].Index })
	shares = shares[:threshold]

	indices := make([]int, 0, len(shares))
	seen := map[int]struct{}{}
	for _, s := range shares {
		if s.Index <= 0 || len(s.Share) != 48 {
			return nil, ErrInvalid
		}
		if _, ok := seen[s.Index]; ok {
			return nil, ErrInvalid
		}
		seen[s.Index] = struct{}{}
		indices = append(indices, s.Index)
	}

	// Combine: C1^s = Σ λ_i * (C1^{s_i}).
	acc := new(blst.P1)
	for _, sh := range shares {
		coeff, err := lagrangeAtZeroScalar(sh.Index, indices)
		if err != nil {
			return nil, err
		}
		var aff blst.P1Affine
		if aff.Uncompress(sh.Share) == nil {
			return nil, ErrInvalid
		}
		var p blst.P1
		p.FromAffine(&aff)
		p.MultAssign(coeff)
		acc.AddAssign(&p)
	}

	// m = C2 - C1^s
	var c2Aff blst.P1Affine
	if c2Aff.Uncompress(ct.C2) == nil {
		return nil, ErrInvalid
	}
	var c2 blst.P1
	c2.FromAffine(&c2Aff)
	c2.SubAssign(acc)
	return c2.ToAffine().Compress(), nil
}

func lagrangeAtZeroScalar(i int, indices []int) (*blst.Scalar, error) {
	xi := scalarFromInt(i)
	num := scalarFromInt(1)
	den := scalarFromInt(1)
	zero := scalarFromInt(0)
	for _, j := range indices {
		if j == i {
			continue
		}
		xj := scalarFromInt(j)
		neg, ok := zero.Sub(xj)
		if !ok {
			return nil, ErrInvalid
		}
		num, ok = num.Mul(neg)
		if !ok {
			return nil, ErrInvalid
		}
		diff, ok := xi.Sub(xj)
		if !ok {
			return nil, ErrInvalid
		}
		den, ok = den.Mul(diff)
		if !ok {
			return nil, ErrInvalid
		}
	}
	inv := den.Inverse()
	out, ok := num.Mul(inv)
	if !ok {
		return nil, ErrInvalid
	}
	return out, nil
}

func scalarFromInt(v int) *blst.Scalar {
	var buf [blst.BLST_SCALAR_BYTES]byte
	binary.BigEndian.PutUint64(buf[len(buf)-8:], uint64(v))
	var s blst.Scalar
	_ = s.FromBEndian(buf[:])
	return &s
}

func randScalar() (*blst.Scalar, error) {
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ikm); err != nil {
		return nil, err
	}
	s := blst.KeyGen(ikm)
	if s == nil {
		return nil, ErrInvalid
	}
	return s, nil
}
