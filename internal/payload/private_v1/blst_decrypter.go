//go:build blst

package private_v1

import (
	"errors"

	"github.com/zmlAEQ/Aequa-network/internal/beast"
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
	tx, ok := p.(*PrivateTx)
	if !ok {
		return p, nil
	}
	if tx.TargetHeight == 0 {
		return nil, errors.New("missing target height")
	}
	pt, err := beast.Decrypt(tx.Ciphertext)
	if err != nil {
		return nil, err
	}
	// The decrypted payload mapping into sortable payloads (plaintext_v1 or
	// auction_bid_v1) is performed downstream via the registered private
	// decrypter hook; here we only ensure ciphertext can be opened under the
	// configured BEAST engine.
	if len(pt) == 0 {
		return nil, errors.New("empty plaintext")
	}
	return p, nil
}
