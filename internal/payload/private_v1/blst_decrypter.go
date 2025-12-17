//go:build blst

package private_v1

import (
	"errors"

	"github.com/zmlAEQ/Aequa-network/internal/beast"
	"github.com/zmlAEQ/Aequa-network/internal/beast/ibe"
	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// EnableBLSTDecrypt installs a blst-backed decrypter (build tag blst required).
// Note: crypto wiring is stubbed; integrate real threshold decrypt in a follow-up.
func EnableBLSTDecrypt(conf Config) error {
	if len(conf.GroupPubKey) == 0 {
		return payload.ErrPrivateInvalid
	}
	// Optional threshold mode: node holds a secret share and releases one share per height.
	if conf.Mode == "threshold" || conf.Threshold > 0 {
		if conf.Threshold <= 0 || conf.Index <= 0 || len(conf.Share) != 32 {
			return errors.New("invalid threshold config")
		}
		enableThreshold(conf.Index, conf.Threshold, conf.Share)
	} else {
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
	// Threshold path: use per-height IBE private key derived from shares.
	if d.conf.Mode == "threshold" || d.conf.Threshold > 0 {
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
