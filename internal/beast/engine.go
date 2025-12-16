package beast

// Engine defines the minimal BEAST crypto surface used by the rest of the
// system. Implementations may provide threshold encryption/decryption or a
// simplified single-key variant; the interface is kept small on purpose.
type Engine interface {
	// Encrypt returns an encoded ciphertext for the provided plaintext.
	Encrypt(msg []byte) ([]byte, error)
	// Decrypt returns the recovered plaintext for a full ciphertext. For
	// threshold flows, this is expected to be called after aggregation.
	Decrypt(cipher []byte) ([]byte, error)
}

var current Engine = noopEngine{}

// SetEngine installs a global Engine implementation. Passing nil resets the
// engine to a disabled noop implementation.
func SetEngine(e Engine) {
	if e == nil {
		current = noopEngine{}
		return
	}
	current = e
}

// Encrypt uses the globally registered Engine to encrypt a message.
func Encrypt(msg []byte) ([]byte, error) {
	return current.Encrypt(msg)
}

// Decrypt uses the globally registered Engine to decrypt a ciphertext.
func Decrypt(cipher []byte) ([]byte, error) {
	return current.Decrypt(cipher)
}

type noopEngine struct{}

func (noopEngine) Encrypt([]byte) ([]byte, error)  { return nil, ErrNotEnabled }
func (noopEngine) Decrypt([]byte) ([]byte, error)  { return nil, ErrNotEnabled }

