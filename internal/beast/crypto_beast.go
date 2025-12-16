//go:build beast

package beast

// NOTE: Placeholder for real BEAST threshold flows using blst or other crypto
// libs. This file keeps the partial/aggregate APIs wired behind the 'beast'
// build tag without changing default builds.

// PartialDecrypt performs placeholder partial decrypt (stub).
func PartialDecrypt(cipher []byte) ([]byte, error) {
	// TODO: replace with real partial decryption
	return cipher, nil
}

// AggregateDecrypt performs placeholder aggregate decrypt (stub).
func AggregateDecrypt(parts [][]byte) ([]byte, error) {
	// TODO: replace with real aggregate decryption
	if len(parts) == 0 {
		return nil, nil
	}
	return parts[0], nil
}
