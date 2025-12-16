//go:build beast

package beast

// NOTE: Placeholder for real BEAST implementation using blst or other crypto libs.
// This file exists under build tag "beast" to allow future integration without
// impacting default builds.

// engineBeast is a minimal Engine used when the 'beast' build tag is active.
// It currently performs a reversible echo to keep the path wired; callers
// should treat it as insecure and rely on feature flags for isolation.
type engineBeast struct{}

func (engineBeast) Encrypt(msg []byte) ([]byte, error) {
	// TODO: replace with real threshold encryption
	out := make([]byte, len(msg))
	copy(out, msg)
	return out, nil
}

func (engineBeast) Decrypt(cipher []byte) ([]byte, error) {
	// TODO: replace with real threshold decryption
	out := make([]byte, len(cipher))
	copy(out, cipher)
	return out, nil
}

func init() {
	// When built with the 'beast' tag, install the placeholder engine so that
	// Encrypt/Decrypt delegate to engineBeast instead of the disabled stub.
	SetEngine(engineBeast{})
}

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
