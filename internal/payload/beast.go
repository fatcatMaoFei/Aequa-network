package payload

import "github.com/zmlAEQ/Aequa-network/pkg/metrics"

// PrivateDecrypter defines how private_v1 payloads are decrypted/mapped into sortable payloads.
// A no-op default keeps txs intact; real BEAST integration should replace this via SetPrivateDecrypter.
type PrivateDecrypter interface {
	Decrypt(p Payload) (Payload, error)
}

var privateDecrypter PrivateDecrypter = noopDecrypter{}

// SetPrivateDecrypter overrides the global decrypter used by the builder when BEAST is enabled.
func SetPrivateDecrypter(d PrivateDecrypter) {
	if d != nil {
		privateDecrypter = d
	}
}

type noopDecrypter struct{}

func (noopDecrypter) Decrypt(p Payload) (Payload, error) { return p, nil }

func recordDecryptMetric(result string) {
	metrics.Inc("beast_decrypt_total", map[string]string{"result": result})
}
