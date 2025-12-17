//go:build blst

package dkg

import (
	"testing"

	blst "github.com/supranational/blst/bindings/go"
	"github.com/zmlAEQ/Aequa-network/internal/beast/ibe"
)

func TestSimulateFeldmanDKG_ReconstructSecretAndDecrypt(t *testing.T) {
	n := 4
	thr := 3
	gpk, shares, err := SimulateFeldmanDKG(n, thr)
	if err != nil {
		t.Fatalf("SimulateFeldmanDKG: %v", err)
	}
	if len(gpk) != 48 {
		t.Fatalf("group pubkey len: got %d", len(gpk))
	}
	if len(shares) != n {
		t.Fatalf("shares: got %d", len(shares))
	}

	// Reconstruct master secret scalar at x=0 from any thr shares and validate gpk matches g1^s.
	ss := make([]scalarShare, 0, thr)
	for i := 1; i <= thr; i++ {
		var sc blst.Scalar
		if sc.Deserialize(shares[i]) == nil {
			t.Fatalf("bad scalar share")
		}
		ss = append(ss, scalarShare{Index: i, Value: &sc})
	}
	sec, err := combineScalarSharesAtZero(ss, thr)
	if err != nil {
		t.Fatalf("combineScalarSharesAtZero: %v", err)
	}
	if sec == nil {
		t.Fatalf("nil secret")
	}
	// Sanity: gpk == g1^sec.
	if got := blst.P1Generator().Mult(sec).ToAffine().Compress(); string(got) != string(gpk) {
		t.Fatalf("group pubkey mismatch")
	}

	// Encrypt/decrypt via threshold IBE shares to ensure BEAST path is compatible.
	id := ibe.IdentityForHeight(123)
	msg := []byte("hello beast dkg")
	eph, ct, err := ibe.Encrypt(gpk, id, msg)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Derive decryption shares for the identity using participant secret shares.
	decShares := make([]ibe.Share, 0, thr)
	for i := 1; i <= thr; i++ {
		sh, err := ibe.DeriveShare(shares[i], id)
		if err != nil {
			t.Fatalf("DeriveShare: %v", err)
		}
		decShares = append(decShares, ibe.Share{Index: i, Value: sh})
	}

	priv, err := ibe.CombineShares(decShares, thr)
	if err != nil {
		t.Fatalf("CombineShares: %v", err)
	}
	pt, err := ibe.Decrypt(priv, eph, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(pt) != string(msg) {
		t.Fatalf("plaintext mismatch: got %q want %q", string(pt), string(msg))
	}
}
