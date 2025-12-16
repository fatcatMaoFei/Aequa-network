//go:build beast

package beast

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// symmetricEngine is a simple AES-GCM engine keyed from opaque group key
// material. It is intended as a non-threshold MVP; callers should not treat
// it as the final BEAST construction.
type symmetricEngine struct {
	key [32]byte
}

// NewSymmetricEngine constructs an Engine from arbitrary key material.
// The key is hashed via SHA-256 to obtain a 32-byte AES key. An empty
// key results in a disabled engine.
func NewSymmetricEngine(key []byte) Engine {
	if len(key) == 0 {
		return noopEngine{}
	}
	sum := sha256.Sum256(key)
	return symmetricEngine{key: sum}
}

func (e symmetricEngine) Encrypt(msg []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, msg, nil)
	out := make([]byte, len(nonce)+len(ct))
	copy(out, nonce)
	copy(out[len(nonce):], ct)
	return out, nil
}

func (e symmetricEngine) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

