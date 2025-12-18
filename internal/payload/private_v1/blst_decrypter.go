//go:build blst

package private_v1

import (
	"context"
	"errors"

	"github.com/zmlAEQ/Aequa-network/internal/beast"
	"github.com/zmlAEQ/Aequa-network/internal/beast/bte"
	"github.com/zmlAEQ/Aequa-network/internal/beast/ibe"
	"github.com/zmlAEQ/Aequa-network/internal/beast/pprf"
	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// EnableBLSTDecrypt installs a blst-backed decrypter (build tag blst required).
// Note: crypto wiring is stubbed; integrate real threshold decrypt in a follow-up.
func EnableBLSTDecrypt(conf Config) error {
	if len(conf.GroupPubKey) == 0 {
		return payload.ErrPrivateInvalid
	}
	switch conf.Mode {
	case "threshold":
		// Threshold mode: node holds a secret share and releases one share per height.
		if conf.Threshold <= 0 || conf.Index <= 0 || len(conf.Share) != 32 {
			return errors.New("invalid threshold config")
		}
		enableThreshold(conf.Index, conf.Threshold, conf.Share)
	case "batched":
		// Batched BEAST uses BTE+PPRF and a local decrypt share; no extra wiring here.
	default:
		// Symmetric MVP path for non-threshold modes.
		beast.SetEngine(beast.NewSymmetricEngine(conf.GroupPubKey))
	}
	payload.SetPrivateDecrypter(blstDecrypter{conf: conf})
	return nil
}

type blstDecrypter struct {
	conf Config
}

func (d blstDecrypter) Decrypt(h payload.BlockHeader, p payload.Payload) (payload.Payload, error) {
	tx, ok := p.(*PrivateTx)
	if !ok {
		return p, nil
	}
	if tx.TargetHeight == 0 {
		return nil, payload.ErrPrivateInvalid
	}
	if h.Height < tx.TargetHeight {
		return nil, payload.ErrPrivateEarly
	}
	switch d.conf.Mode {
	case "threshold":
		return decryptThresholdIBE(d.conf, tx)
	case "batched":
		return decryptBatched(d.conf, tx)
	default:
		// Symmetric MVP path.
		pt, err := beast.Decrypt(tx.Ciphertext)
		if err != nil {
			return nil, payload.ErrPrivateCipher
		}
		if len(pt) == 0 {
			return nil, payload.ErrPrivateEmpty
		}
		pl, err := decodeEnvelopeBytes(pt)
		if err != nil {
			return nil, payload.ErrPrivateDecode
		}
		return pl, nil
	}
}

// decryptThresholdIBE preserves the existing per-height IBE threshold path.
func decryptThresholdIBE(conf Config, tx *PrivateTx) (payload.Payload, error) {
	if len(tx.EphemeralKey) == 0 || len(tx.Ciphertext) == 0 {
		return nil, payload.ErrPrivateInvalid
	}
	// Ensure local share is available for this height (best-effort).
	maybeEnsureShare(tx.TargetHeight)
	pk, ok := thresholdPrivateKey(tx.TargetHeight)
	if !ok {
		return nil, payload.ErrPrivateNotReady
	}
	pt, err := ibe.Decrypt(pk, tx.EphemeralKey, tx.Ciphertext)
	if err != nil {
		if err == ibe.ErrInvalidPoint {
			return nil, payload.ErrPrivateInvalid
		}
		return nil, payload.ErrPrivateCipher
	}
	if len(pt) == 0 {
		return nil, payload.ErrPrivateEmpty
	}
	pl, err := decodeEnvelopeBytes(pt)
	if err != nil {
		return nil, payload.ErrPrivateDecode
	}
	return pl, nil
}

// decryptBatched implements a single-node batched BEAST decrypt path using
// BTE+PPRF. It expects BatchIndex>0, a punctured key, and a 96-byte
// EphemeralKey (C1||C2). Threshold is effectively 1 (local share only).
func decryptBatched(conf Config, tx *PrivateTx) (payload.Payload, error) {
	if conf.BatchN <= 0 {
		return nil, payload.ErrPrivateInvalid
	}
	if len(conf.Share) != 32 || conf.Index <= 0 {
		return nil, payload.ErrPrivateInvalid
	}
	if len(tx.Ciphertext) == 0 || len(tx.EphemeralKey) != 96 || tx.BatchIndex == 0 || len(tx.PuncturedKey) != 48 {
		return nil, payload.ErrPrivateInvalid
	}
	if int(tx.BatchIndex) > conf.BatchN {
		return nil, payload.ErrPrivateInvalid
	}
	pp, err := pprf.SetupLinearDeterministic(conf.BatchN, conf.GroupPubKey)
	if err != nil {
		return nil, payload.ErrPrivateCipher
	}
	ct := bte.KeyCiphertext{
		C1: append([]byte(nil), tx.EphemeralKey[:48]...),
		C2: append([]byte(nil), tx.EphemeralKey[48:]...),
	}
	// Compute and locally record our partial decrypt share; this will be
	// gossiped via P2P when BEAST share publisher is configured.
	sh, err := bte.PartialDecrypt(ct, conf.Share, conf.Index)
	if err != nil {
		return nil, payload.ErrPrivateCipher
	}
	recordBatchedShare(tx.TargetHeight, tx.BatchIndex, sh.Index, sh.Share)
	enabled, _, _, _, pub := thresholdParams()
	if enabled && pub != nil && markShareSent(tx.TargetHeight) {
		// For batched flows we multiplex shares by batch index at the same
		// height via the payload.Ciphertext/BatchIndex; the wire message
		// remains generic to avoid metric/log drift.
		_ = pub(context.Background(), tx.TargetHeight, sh.Index, sh.Share)
	}

	// Collect t out-of-n partial decrypt shares for this (height,batch)
	// and recover g^k via BTE's threshold combine.
	shared := snapshotBatchedShares(tx.TargetHeight, tx.BatchIndex)
	if len(shared) < conf.Threshold {
		return nil, payload.ErrPrivateNotReady
	}
	shares := make([]bte.PartialDecryptShare, 0, len(shared))
	for idx, val := range shared {
		shares = append(shares, bte.PartialDecryptShare{Index: idx, Share: append([]byte(nil), val...)})
	}
	gk, err := bte.DecryptKeyG(ct, shares, conf.Threshold)
	if err != nil {
		return nil, payload.ErrPrivateCipher
	}
	idx := int(tx.BatchIndex)
	punctured := map[int][]byte{idx: append([]byte(nil), tx.PuncturedKey...)}
	prf, err := bte.RecoverPRFAt(pp, gk, idx, punctured)
	if err != nil {
		return nil, payload.ErrPrivateCipher
	}
	pt := bte.RecoverXOR(tx.Ciphertext, prf)
	if len(pt) == 0 {
		return nil, payload.ErrPrivateEmpty
	}
	pl, err := decodeEnvelopeBytes(pt)
	if err != nil {
		return nil, payload.ErrPrivateDecode
	}
	return pl, nil
}
