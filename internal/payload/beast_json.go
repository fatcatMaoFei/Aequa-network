package payload

import (
	"encoding/json"
	"errors"

	auction_v1 "github.com/zmlAEQ/Aequa-network/internal/payload/auction_bid_v1"
	plaintext_v1 "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
	private_v1 "github.com/zmlAEQ/Aequa-network/internal/payload/private_v1"
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

func (jsonDecrypter) Decrypt(p Payload) (Payload, error) {
	tx, ok := p.(*private_v1.PrivateTx)
	if !ok {
		return p, nil
	}
	var env jsonEnvelope
	if err := json.Unmarshal(tx.Ciphertext, &env); err != nil {
		return nil, err
	}
	switch env.Type {
	case "", "plaintext_v1":
		return &plaintext_v1.PlaintextTx{
			From:  env.From,
			Nonce: env.Nonce,
			Gas:   env.Gas,
			Fee:   env.Fee,
		}, nil
	case "auction_bid_v1":
		return &auction_v1.AuctionBidTx{
			From:         env.From,
			Nonce:        env.Nonce,
			Gas:          env.Gas,
			Bid:          env.Bid,
			FeeRecipient: env.FeeRecipient,
		}, nil
	default:
		return nil, errors.New("unsupported private payload type")
	}
}

// EnableLocalJSONDecrypt switches the global decrypter to json-based decoding of Ciphertext.
// This is intended for dev/test where ciphertext already contains JSON of TxEnvelope.
func EnableLocalJSONDecrypt() {
	SetPrivateDecrypter(jsonDecrypter{})
}
