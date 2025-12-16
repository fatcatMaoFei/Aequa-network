package private_v1

import (
	"encoding/json"
	"errors"

	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
	auction_v1 "github.com/zmlAEQ/Aequa-network/internal/payload/auction_bid_v1"
	plaintext_v1 "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

// decodeEnvelopeBytes maps a JSON-encoded envelope into a concrete payload.
// It is shared between dev JSON decrypt and BEAST-backed decrypt paths.
func decodeEnvelopeBytes(b []byte) (payload.Payload, error) {
	var env jsonEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
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

