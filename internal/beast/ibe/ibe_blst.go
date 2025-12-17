//go:build blst

package ibe

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	blst "github.com/supranational/blst/bindings/go"
)

// Domain separation constants.
const (
	hashDST = "EQS/BEAST/v1/IBE"
)

var (
	ErrInvalidPoint = errors.New("invalid point")
	ErrInvalidShare = errors.New("invalid share")
)

// IdentityForHeight returns a deterministic identity byte string for a target height.
func IdentityForHeight(height uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], height)
	out := make([]byte, 0, len(hashDST)+1+len(b))
	out = append(out, []byte(hashDST)...)
	out = append(out, 0)
	out = append(out, b[:]...)
	return out
}

// Share represents a partial private key share (compressed G2) for an identity.
type Share struct {
	Index int
	Value []byte
}

// DeriveShare computes a per-identity private key share: H(id)^shareScalar.
// shareScalar is expected to be a big-endian scalar (32 bytes).
func DeriveShare(shareScalar []byte, id []byte) ([]byte, error) {
	if len(shareScalar) == 0 {
		return nil, ErrInvalidShare
	}
	var sk blst.Scalar
	if sk.Deserialize(shareScalar) == nil {
		return nil, ErrInvalidShare
	}
	h := blst.HashToG2(id, []byte(hashDST), nil)
	h.MultAssign(&sk)
	return h.ToAffine().Compress(), nil
}

// CombineShares Lagrange-combines shares at x=0 (Shamir) to recover the full
// private key for the identity. The returned key is a compressed G2 element.
func CombineShares(shares []Share, k int) ([]byte, error) {
	if k <= 0 {
		return nil, ErrInvalidShare
	}
	if len(shares) < k {
		return nil, ErrInvalidShare
	}
	// Deterministic subset: smallest k indices.
	sort.Slice(shares, func(i, j int) bool { return shares[i].Index < shares[j].Index })
	shares = shares[:k]

	indices := make([]int, 0, len(shares))
	seen := map[int]struct{}{}
	for _, s := range shares {
		if s.Index <= 0 || len(s.Value) == 0 {
			return nil, ErrInvalidShare
		}
		if _, ok := seen[s.Index]; ok {
			return nil, ErrInvalidShare
		}
		seen[s.Index] = struct{}{}
		indices = append(indices, s.Index)
	}

	acc := new(blst.P2)
	for _, s := range shares {
		coeff, err := lagrangeAtZero(s.Index, indices)
		if err != nil {
			return nil, err
		}
		var aff blst.P2Affine
		if aff.Uncompress(s.Value) == nil {
			return nil, ErrInvalidPoint
		}
		var p blst.P2
		p.FromAffine(&aff)
		p.MultAssign(coeff)
		acc.AddAssign(&p)
	}
	return acc.ToAffine().Compress(), nil
}

func lagrangeAtZero(i int, indices []int) (*blst.Scalar, error) {
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

func scalarFromInt(v int) *blst.Scalar {
	var buf [blst.BLST_SCALAR_BYTES]byte
	binary.BigEndian.PutUint64(buf[len(buf)-8:], uint64(v))
	var s blst.Scalar
	_ = s.FromBEndian(buf[:])
	return &s
}

// Encrypt encrypts msg for identity id under a group public key (compressed G1).
// It returns (ephemeralKey, ciphertext) where ephemeralKey is a compressed G1
// point and ciphertext is nonce||AES-GCM(msg).
func Encrypt(groupPubKey []byte, id []byte, msg []byte) ([]byte, []byte, error) {
	var pkAff blst.P1Affine
	if pkAff.Uncompress(groupPubKey) == nil {
		return nil, nil, ErrInvalidPoint
	}
	// r <-$ Fr
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ikm); err != nil {
		return nil, nil, err
	}
	r := blst.KeyGen(ikm)
	if r == nil {
		return nil, nil, errors.New("bad randomness")
	}
	// U = g1^r
	u := blst.P1Generator().Mult(r)
	uBytes := u.ToAffine().Compress()

	// H = H2(id)
	h := blst.HashToG2(id, []byte(hashDST), nil)
	hAff := h.ToAffine()

	// pk^r
	var pkP1 blst.P1
	pkP1.FromAffine(&pkAff)
	pkR := pkP1.Mult(r)
	pkRAff := pkR.ToAffine()

	// K = e(H(id), pk^r)
	gt := blst.Fp12MillerLoop(hAff, pkRAff)
	gt.FinalExp()
	key := sha256.Sum256(gt.ToBendian())

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct := gcm.Seal(nil, nonce, msg, nil)
	out := make([]byte, 0, len(nonce)+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return uBytes, out, nil
}

// Decrypt decrypts ciphertext using the per-identity private key (compressed G2)
// and ephemeralKey (compressed G1).
func Decrypt(privKey []byte, ephemeralKey []byte, ciphertext []byte) ([]byte, error) {
	var skAff blst.P2Affine
	if skAff.Uncompress(privKey) == nil {
		return nil, ErrInvalidPoint
	}
	var uAff blst.P1Affine
	if uAff.Uncompress(ephemeralKey) == nil {
		return nil, ErrInvalidPoint
	}
	gt := blst.Fp12MillerLoop(&skAff, &uAff)
	gt.FinalExp()
	key := sha256.Sum256(gt.ToBendian())

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:ns]
	ct := ciphertext[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}
