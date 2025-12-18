//go:build blst

package private_v1

import (
	"crypto/rand"
	"encoding/json"
	"testing"

	blst "github.com/supranational/blst/bindings/go"

	"github.com/zmlAEQ/Aequa-network/internal/beast/bte"
	"github.com/zmlAEQ/Aequa-network/internal/beast/pprf"
	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
	plaintext_v1 "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

func TestBatchedDecryptSingleNodeRoundTrip(t *testing.T) {
	// Single-node (threshold=1) batched BEAST config.
	sk, err := randScalarTest()
	if err != nil {
		t.Fatalf("randScalarTest: %v", err)
	}
	var gpk blst.P1Affine
	gpk.From(sk)
	gpkBytes := gpk.Compress()

	conf := Config{
		Mode:        "batched",
		GroupPubKey: gpkBytes,
		Threshold:   1,
		Index:       1,
		Share:       sk.Serialize(),
		BatchN:      4,
	}
	if err := EnableBLSTDecrypt(conf); err != nil {
		t.Fatalf("EnableBLSTDecrypt: %v", err)
	}

	pp, err := pprf.SetupLinearDeterministic(conf.BatchN, conf.GroupPubKey)
	if err != nil {
		t.Fatalf("SetupLinearDeterministic: %v", err)
	}
	key, err := pprf.KeyGen()
	if err != nil {
		t.Fatalf("KeyGen: %v", err)
	}
	idx := 2

	inner := jsonEnvelope{
		Type:  "plaintext_v1",
		From:  "A",
		Nonce: 1,
		Gas:   1,
		Fee:   2,
	}
	pt, _ := json.Marshal(inner)

	prf, err := pprf.Eval(pp, key, idx)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	beta := bte.RecoverXOR(pt, prf)
	ct, err := bte.EncryptKey(conf.GroupPubKey, key)
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	kStar, err := pprf.Puncture(pp, key, idx)
	if err != nil {
		t.Fatalf("Puncture: %v", err)
	}

	tx := &PrivateTx{
		From:         inner.From,
		Nonce:        inner.Nonce,
		Ciphertext:   beta,
		EphemeralKey: append(append([]byte(nil), ct.C1...), ct.C2...),
		TargetHeight: 10,
		BatchIndex:   uint64(idx),
		PuncturedKey: kStar,
	}

	hdr := payload.BlockHeader{Height: 10}
	d := blstDecrypter{conf: conf}
	out, err := d.Decrypt(hdr, tx)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	ptx, ok := out.(*plaintext_v1.PlaintextTx)
	if !ok {
		t.Fatalf("unexpected payload type %T", out)
	}
	if ptx.From != inner.From || ptx.Nonce != inner.Nonce || ptx.Gas != inner.Gas || ptx.Fee != inner.Fee {
		t.Fatalf("roundtrip mismatch: got %+v", ptx)
	}
}

// randScalarTest mirrors the helper in pprf but is local to tests.
func randScalarTest() (*blst.Scalar, error) {
	ikm := make([]byte, 32)
	if _, err := rand.Read(ikm); err != nil {
		return nil, err
	}
	s := blst.KeyGen(ikm)
	if s == nil {
		return nil, pprf.ErrInvalidKey
	}
	return s, nil
}
