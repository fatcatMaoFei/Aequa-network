//go:build blst

package pprf

import (
	"bytes"
	"testing"

	blst "github.com/supranational/blst/bindings/go"
)

func TestLinearPPRF_PuncturedEvalMatchesEval(t *testing.T) {
	pp, err := SetupLinear(8)
	if err != nil {
		t.Fatalf("SetupLinear: %v", err)
	}
	k, err := KeyGen()
	if err != nil {
		t.Fatalf("KeyGen: %v", err)
	}
	iStar := 3
	kStar, err := Puncture(pp, k, iStar)
	if err != nil {
		t.Fatalf("Puncture: %v", err)
	}
	for i := 1; i <= pp.N; i++ {
		if i == iStar {
			if _, err := PuncturedEval(pp, kStar, iStar, i); err == nil {
				t.Fatalf("expected punctured eval error for i=%d", i)
			}
			continue
		}
		y1, err := Eval(pp, k, i)
		if err != nil {
			t.Fatalf("Eval(i=%d): %v", i, err)
		}
		y2, err := PuncturedEval(pp, kStar, iStar, i)
		if err != nil {
			t.Fatalf("PuncturedEval(i=%d): %v", i, err)
		}
		if !bytes.Equal(y1, y2) {
			t.Fatalf("mismatch for i=%d", i)
		}
	}
}

func TestLinearPPRF_KeyHomomorphic(t *testing.T) {
	pp, err := SetupLinear(8)
	if err != nil {
		t.Fatalf("SetupLinear: %v", err)
	}
	k1, err := KeyGen()
	if err != nil {
		t.Fatalf("KeyGen(k1): %v", err)
	}
	k2, err := KeyGen()
	if err != nil {
		t.Fatalf("KeyGen(k2): %v", err)
	}
	k12, err := AddKeys(k1, k2)
	if err != nil {
		t.Fatalf("AddKeys: %v", err)
	}

	i := 2
	exp := pp.N + 1 + i
	if exp <= 0 || exp >= len(pp.G1Pows) {
		t.Fatalf("bad exp index %d", exp)
	}
	p1k1, err := mulG1(pp.G1Pows[exp], k1)
	if err != nil {
		t.Fatalf("mulG1(k1): %v", err)
	}
	p1k2, err := mulG1(pp.G1Pows[exp], k2)
	if err != nil {
		t.Fatalf("mulG1(k2): %v", err)
	}
	p1sum, err := addG1(p1k1, p1k2)
	if err != nil {
		t.Fatalf("addG1: %v", err)
	}
	p1k12, err := mulG1(pp.G1Pows[exp], k12)
	if err != nil {
		t.Fatalf("mulG1(k12): %v", err)
	}
	if !bytes.Equal(p1sum, p1k12) {
		t.Fatalf("G1 homomorphism mismatch")
	}
}

func TestLinearPPRF_EvalFromGKMatchesEval(t *testing.T) {
	pp, err := SetupLinear(8)
	if err != nil {
		t.Fatalf("SetupLinear: %v", err)
	}
	k, err := KeyGen()
	if err != nil {
		t.Fatalf("KeyGen: %v", err)
	}
	var sk blst.Scalar
	if sk.Deserialize(k) == nil {
		t.Fatalf("Deserialize(k)")
	}
	gk := blst.P1Generator().Mult(&sk).ToAffine().Compress()
	i := 4
	y1, err := Eval(pp, k, i)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	y2, err := EvalFromGK(pp, gk, i)
	if err != nil {
		t.Fatalf("EvalFromGK: %v", err)
	}
	if !bytes.Equal(y1, y2) {
		t.Fatalf("EvalFromGK mismatch")
	}
}

func mulG1(base []byte, scalar []byte) ([]byte, error) {
	if len(base) != 48 || len(scalar) != 32 {
		return nil, ErrInvalidParams
	}
	var k blst.Scalar
	if k.Deserialize(scalar) == nil {
		return nil, ErrInvalidKey
	}
	var baseAff blst.P1Affine
	if baseAff.Uncompress(base) == nil {
		return nil, ErrInvalidParams
	}
	var p blst.P1
	p.FromAffine(&baseAff)
	out := p.Mult(&k)
	return out.ToAffine().Compress(), nil
}

func addG1(aBytes []byte, bBytes []byte) ([]byte, error) {
	if len(aBytes) != 48 || len(bBytes) != 48 {
		return nil, ErrInvalidParams
	}
	var aAff blst.P1Affine
	if aAff.Uncompress(aBytes) == nil {
		return nil, ErrInvalidParams
	}
	var bAff blst.P1Affine
	if bAff.Uncompress(bBytes) == nil {
		return nil, ErrInvalidParams
	}
	var a blst.P1
	var b blst.P1
	a.FromAffine(&aAff)
	b.FromAffine(&bAff)
	a.AddAssign(&b)
	return a.ToAffine().Compress(), nil
}
