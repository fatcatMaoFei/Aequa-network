//go:build beast

package beast

import (
	"bytes"
	"errors"
)

// NOTE: Threshold decrypt skeleton for BEAST flows. For the current symmetric
// MVP, each participant is able to perform a full decrypt using the shared
// Engine key; PartialDecrypt therefore returns a plaintext "share", and
// AggregateDecrypt checks that all provided shares agree before returning the
// reconstructed plaintext. This keeps the API and call-sites compatible with a
// future true threshold implementation without changing default behaviour.

// PartialDecrypt performs a single-party decrypt using the global Engine.
// Callers are expected to collect shares from multiple parties and pass them
// to AggregateDecrypt.
func PartialDecrypt(cipher []byte) ([]byte, error) {
	pt, err := Decrypt(cipher)
	if err != nil {
		return nil, err
	}
	cp := make([]byte, len(pt))
	copy(cp, pt)
	return cp, nil
}

// AggregateDecrypt combines multiple partial decrypt shares into a single
// plaintext. For the symmetric MVP, all shares must match byte-for-byte; any
// mismatch results in an error.
func AggregateDecrypt(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, ErrNotEnabled
	}
	var base []byte
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		if base == nil {
			base = make([]byte, len(p))
			copy(base, p)
			continue
		}
		if !bytes.Equal(base, p) {
			return nil, errors.New("inconsistent partial decrypt shares")
		}
	}
	if base == nil {
		return nil, ErrNotEnabled
	}
	return base, nil
}
