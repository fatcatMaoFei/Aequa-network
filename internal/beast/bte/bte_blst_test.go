//go:build blst

package bte

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	blst "github.com/supranational/blst/bindings/go"

	"github.com/zmlAEQ/Aequa-network/internal/beast/pprf"
)

func TestThresholdElGamal_BatchDecryptAndPRFRecovery(t *testing.T) {
	const (
		n  = 4
		tk = 3
		b  = 3
	)

	// Threshold key material.
	s, err := randScalar()
	if err != nil {
		t.Fatalf("randScalar(s): %v", err)
	}
	pk := blst.P1Generator().Mult(s).ToAffine().Compress()
	shares, err := shamirShares(*s, tk, n)
	if err != nil {
		t.Fatalf("shamirShares: %v", err)
	}

	// Batch keys {k_i}.
	keys := make([][]byte, 0, b)
	cts := make([]KeyCiphertext, 0, b)
	expectedGK := new(blst.P1)
	for i := 1; i <= b; i++ {
		ki, err := randKeyScalar()
		if err != nil {
			t.Fatalf("randKeyScalar: %v", err)
		}
		keys = append(keys, ki)
		ct, err := EncryptKey(pk, ki)
		if err != nil {
			t.Fatalf("EncryptKey: %v", err)
		}
		cts = append(cts, ct)

		var sk blst.Scalar
		if sk.Deserialize(ki) == nil {
			t.Fatalf("Deserialize(k)")
		}
		expectedGK.AddAssign(blst.P1Generator().Mult(&sk))
	}
	expectedGKBytes := expectedGK.ToAffine().Compress()

	ctAgg, err := AddCiphertexts(cts)
	if err != nil {
		t.Fatalf("AddCiphertexts: %v", err)
	}

	// Partial decrypt shares from first tk participants.
	var decShares []PartialDecryptShare
	for i := 1; i <= tk; i++ {
		sh, err := PartialDecrypt(ctAgg, shares[i], i)
		if err != nil {
			t.Fatalf("PartialDecrypt(%d): %v", i, err)
		}
		decShares = append(decShares, sh)
	}
	gkOut, err := DecryptKeyG(ctAgg, decShares, tk)
	if err != nil {
		t.Fatalf("DecryptKeyG: %v", err)
	}
	if !bytes.Equal(gkOut, expectedGKBytes) {
		t.Fatalf("g^k mismatch")
	}

	// PRF setup for domain [1..b], and punctured keys per tx index.
	pp, err := pprf.SetupLinear(b)
	if err != nil {
		t.Fatalf("pprf.SetupLinear: %v", err)
	}
	punctured := make(map[int][]byte, b)
	for i := 1; i <= b; i++ {
		kStar, err := pprf.Puncture(pp, keys[i-1], i)
		if err != nil {
			t.Fatalf("pprf.Puncture(%d): %v", i, err)
		}
		punctured[i] = kStar
	}
	// For each i, RecoverPRFAt should match direct Eval(k_i,i), and XOR unmask should round-trip.
	for i := 1; i <= b; i++ {
		want, err := pprf.Eval(pp, keys[i-1], i)
		if err != nil {
			t.Fatalf("pprf.Eval(%d): %v", i, err)
		}
		got, err := RecoverPRFAt(pp, gkOut, i, punctured)
		if err != nil {
			t.Fatalf("RecoverPRFAt(%d): %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("prf mismatch for i=%d", i)
		}
		// beta = msg XOR H(prf)
		msg := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, msg); err != nil {
			t.Fatalf("rand msg: %v", err)
		}
		beta := RecoverXOR(msg, want)  // mask
		plain := RecoverXOR(beta, got) // unmask
		if !bytes.Equal(plain, msg) {
			t.Fatalf("xor roundtrip mismatch for i=%d", i)
		}
	}
}

// shamirShares returns shares[1..n] for secret s at x=0 using a random
// polynomial of degree (t-1). share bytes are 32-byte scalars.
func shamirShares(secret blst.Scalar, t int, n int) (map[int][]byte, error) {
	if t <= 0 || n <= 0 || t > n {
		return nil, ErrInvalid
	}
	coeffs := make([]*blst.Scalar, t)
	coeffs[0] = &secret
	for i := 1; i < t; i++ {
		s, err := randScalar()
		if err != nil {
			return nil, err
		}
		coeffs[i] = s
	}
	out := make(map[int][]byte, n)
	for i := 1; i <= n; i++ {
		v, err := evalPoly(coeffs, i)
		if err != nil {
			return nil, err
		}
		out[i] = v.Serialize()
	}
	return out, nil
}

func evalPoly(coeffs []*blst.Scalar, x int) (*blst.Scalar, error) {
	if len(coeffs) == 0 {
		return nil, ErrInvalid
	}
	xs := scalarFromInt(x)
	res := *coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		if _, ok := (&res).MulAssign(xs); !ok {
			return nil, ErrInvalid
		}
		if _, ok := (&res).AddAssign(coeffs[i]); !ok {
			return nil, ErrInvalid
		}
	}
	return &res, nil
}

func randKeyScalar() ([]byte, error) {
	s, err := randScalar()
	if err != nil {
		return nil, err
	}
	return s.Serialize(), nil
}
