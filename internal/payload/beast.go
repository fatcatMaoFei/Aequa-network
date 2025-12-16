package payload

import (
	"errors"

	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// PrivateDecrypter defines how private_v1 payloads are decrypted/mapped into sortable payloads.
// A no-op default keeps txs intact; real BEAST integration should replace this via SetPrivateDecrypter.
type PrivateDecrypter interface {
	Decrypt(h BlockHeader, p Payload) (Payload, error)
}

var privateDecrypter PrivateDecrypter = noopDecrypter{}

// SetPrivateDecrypter overrides the global decrypter used by the builder when BEAST is enabled.
func SetPrivateDecrypter(d PrivateDecrypter) {
	if d != nil {
		privateDecrypter = d
	}
}

type noopDecrypter struct{}

func (noopDecrypter) Decrypt(_ BlockHeader, p Payload) (Payload, error) { return p, nil }

// Sentinel errors used to categorize BEAST/private decrypt outcomes for metrics.
var (
	ErrPrivateInvalid = errors.New("private tx invalid")
	ErrPrivateEarly   = errors.New("private tx early")
	ErrPrivateCipher  = errors.New("private tx cipher error")
	ErrPrivateEmpty   = errors.New("private tx empty")
	ErrPrivateDecode  = errors.New("private tx decode error")
)

func recordDecryptMetric(result string) {
	metrics.Inc("beast_decrypt_total", map[string]string{"result": result})
}
