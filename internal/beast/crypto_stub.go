//go:build !beast

package beast

// Encrypt performs placeholder encryption (stub).
func Encrypt(msg []byte) ([]byte, error) { return nil, ErrNotEnabled }

// PartialDecrypt performs placeholder partial decrypt (stub).
func PartialDecrypt(cipher []byte) ([]byte, error) { return nil, ErrNotEnabled }

// AggregateDecrypt performs placeholder aggregate decrypt (stub).
func AggregateDecrypt(parts [][]byte) ([]byte, error) { return nil, ErrNotEnabled }
