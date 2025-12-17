//go:build blst

package private_v1

import (
	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// EnableThresholdPending installs a placeholder decrypter that classifies private_v1
// payloads as not_ready until a real BEAST engine / threshold share is configured.
func EnableThresholdPending() {
	payload.SetPrivateDecrypter(pendingDecrypter{})
}

type pendingDecrypter struct{}

func (pendingDecrypter) Decrypt(h payload.BlockHeader, p payload.Payload) (payload.Payload, error) {
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
	return nil, payload.ErrPrivateNotReady
}

