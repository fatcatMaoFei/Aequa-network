//go:build blst

package private_v1

import (
	"errors"

	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// EnableBLSTDecrypt installs a blst-backed decrypter (build tag blst required).
// Note: crypto wiring is stubbed; integrate real threshold decrypt in a follow-up.
func EnableBLSTDecrypt(conf Config) error {
	if len(conf.GroupPubKey) == 0 {
		return errors.New("missing group pubkey")
	}
	payload.SetPrivateDecrypter(blstDecrypter{conf: conf})
	return nil
}

type blstDecrypter struct {
	conf Config
}

func (d blstDecrypter) Decrypt(p payload.Payload) (payload.Payload, error) {
	// TODO: plug real threshold decrypt with blst; currently a guarded stub.
	tx, ok := p.(*PrivateTx)
	if !ok {
		return p, nil
	}
	if tx.TargetHeight == 0 {
		return nil, errors.New("missing target height")
	}
	return nil, errors.New("blst decrypt not implemented")
}
