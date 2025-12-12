package wire

import (
    qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
)

// Topic names for pubsub channels (stable identifiers).
const (
	TopicQBFT      = "aequa/qbft/v1"
	TopicTx        = "aequa/tx/v1"
	TopicTxPrivate = "aequa/tx/private/v1"
)

// QBFT is the wire-format counterpart of qbft.Message.
// JSON encoding uses lower_snake_case keys and base64 for []byte fields.
type QBFT struct {
    ID      string `json:"id"`
    From    string `json:"from"`
    Height  uint64 `json:"height"`
    Round   uint64 `json:"round"`
    Type    string `json:"type"`
    Payload []byte `json:"payload,omitempty"`
    TraceID string `json:"trace_id,omitempty"`
    Sig     []byte `json:"sig,omitempty"`
}

// FromInternal converts an internal qbft.Message to its wire form.
func FromInternal(msg qbft.Message) QBFT {
    return QBFT{
        ID:      msg.ID,
        From:    msg.From,
        Height:  msg.Height,
        Round:   msg.Round,
        Type:    string(msg.Type),
        Payload: msg.Payload,
        TraceID: msg.TraceID,
        Sig:     msg.Sig,
    }
}

// ToInternal converts a wire-form QBFT message to the internal type.
func (w QBFT) ToInternal() qbft.Message {
    return qbft.Message{
        ID:      w.ID,
        From:    w.From,
        Height:  w.Height,
        Round:   w.Round,
        Type:    qbft.Type(w.Type),
        Payload: w.Payload,
        TraceID: w.TraceID,
        Sig:     w.Sig,
    }
}
