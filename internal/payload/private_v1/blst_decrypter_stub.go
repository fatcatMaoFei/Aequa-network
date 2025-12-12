//go:build !blst

package private_v1

import (
	"errors"

	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// EnableBLSTDecrypt is a stub when blst tag is not enabled.
func EnableBLSTDecrypt(_ Config) error {
	return errors.New("blst build tag not enabled")
}

// blstDecrypter is unused without the blst tag; defined to satisfy interfaces.
type blstDecrypter struct {
	conf Config
}

func (blstDecrypter) Decrypt(p payload.Payload) (payload.Payload, error) {
	return p, errors.New("blst decrypt disabled")
}
