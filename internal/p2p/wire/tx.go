package wire

import (
	"encoding/json"

	"github.com/zmlAEQ/Aequa-network/internal/payload"
	auction "github.com/zmlAEQ/Aequa-network/internal/payload/auction_bid_v1"
	plaintext "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

// Topic name for transaction gossip.
const TopicTx = "aequa/tx/v1"

const (
	TypePlaintextV1  = "plaintext_v1"
	TypeAuctionBidV1 = "auction_bid_v1"
)

// TxEnvelope is a wire-format transaction that supports multiple tx types.
type TxEnvelope struct {
	Type         string `json:"type"` // plaintext_v1 (default) or auction_bid_v1
	From         string `json:"from"`
	Nonce        uint64 `json:"nonce"`
	Gas          uint64 `json:"gas"`
	Fee          uint64 `json:"fee,omitempty"`
	Bid          uint64 `json:"bid,omitempty"`
	FeeRecipient string `json:"fee_recipient,omitempty"`
	Sig          []byte `json:"sig,omitempty"`
}

// TxFromInternal converts a generic payload to a wire tx if supported.
func TxFromInternal(pl payload.Payload) (TxEnvelope, bool) {
	switch tx := pl.(type) {
	case *plaintext.PlaintextTx:
		if tx.Type() != TypePlaintextV1 {
			return TxEnvelope{}, false
		}
		return TxEnvelope{
			Type:  tx.Type(),
			From:  tx.From,
			Nonce: tx.Nonce,
			Gas:   tx.Gas,
			Fee:   tx.Fee,
			Sig:   tx.Sig,
		}, true
	case *auction.AuctionBidTx:
		if tx.Type() != TypeAuctionBidV1 {
			return TxEnvelope{}, false
		}
		return TxEnvelope{
			Type:         tx.Type(),
			From:         tx.From,
			Nonce:        tx.Nonce,
			Gas:          tx.Gas,
			Bid:          tx.Bid,
			FeeRecipient: tx.FeeRecipient,
			Sig:          tx.Sig,
		}, true
	default:
		return TxEnvelope{}, false
	}
}

// ToInternal converts the wire tx back to a payload instance.
func (w TxEnvelope) ToInternal() payload.Payload {
	if w.Type == "" {
		w.Type = TypePlaintextV1
	}
	switch w.Type {
	case TypePlaintextV1:
		return &plaintext.PlaintextTx{
			From:  w.From,
			Nonce: w.Nonce,
			Gas:   w.Gas,
			Fee:   w.Fee,
			Sig:   w.Sig,
		}
	case TypeAuctionBidV1:
		return &auction.AuctionBidTx{
			From:         w.From,
			Nonce:        w.Nonce,
			Gas:          w.Gas,
			Bid:          w.Bid,
			FeeRecipient: w.FeeRecipient,
			Sig:          w.Sig,
		}
	default:
		return nil
	}
}

// ParseTx decodes JSON into a payload.Payload (without calling Validate).
func ParseTx(b []byte) (payload.Payload, error) {
	var env TxEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	return env.ToInternal(), nil
}
