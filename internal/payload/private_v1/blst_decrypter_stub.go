//go:build !blst

package private_v1

import (
	"errors"
)

// EnableBLSTDecrypt is a stub when blst tag is not enabled.
func EnableBLSTDecrypt(_ Config) error {
	return errors.New("blst build tag not enabled")
}
