package private_v1

import (
	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
)

// jsonEnvelope is a minimal encoding of plaintext/auction tx fields carried inside Ciphertext.
// This is a deterministic local-decrypt helper for testing and non-crypto environments.
type jsonEnvelope struct {
	Type         string `json:"type"`
	From         string `json:"from"`
	Nonce        uint64 `json:"nonce"`
	Gas          uint64 `json:"gas"`
	Fee          uint64 `json:"fee,omitempty"`
	Bid          uint64 `json:"bid,omitempty"`
	FeeRecipient string `json:"fee_recipient,omitempty"`
}

type jsonDecrypter struct{}

func (jsonDecrypter) Decrypt(p payload.Payload) (payload.Payload, error) {
	tx, ok := p.(*PrivateTx)
	if !ok {
		return p, nil
	}
	return decodeEnvelopeBytes(tx.Ciphertext)
}

// EnableLocalJSONDecrypt switches the global decrypter to json-based decoding of Ciphertext.
// This is intended for dev/test where ciphertext already contains JSON of TxEnvelope.
func EnableLocalJSONDecrypt() {
	payload.SetPrivateDecrypter(jsonDecrypter{})
}
