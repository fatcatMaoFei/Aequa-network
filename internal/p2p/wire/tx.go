package wire

import (
    "github.com/zmlAEQ/Aequa-network/internal/payload"
    plaintext "github.com/zmlAEQ/Aequa-network/internal/payload/plaintext_v1"
)

// Topic name for transaction gossip.
const TopicTx = "aequa/tx/v1"

// PlaintextTx is the wire-format counterpart of plaintext_v1.PlaintextTx.
// The Type field is kept for forward compatibility (multiple tx types).
type PlaintextTx struct {
    Type  string `json:"type"` // fixed: "plaintext_v1"
    From  string `json:"from"`
    Nonce uint64 `json:"nonce"`
    Gas   uint64 `json:"gas"`
    Fee   uint64 `json:"fee"`
    Sig   []byte `json:"sig,omitempty"`
}

// TxFromInternal converts a generic payload to a wire tx if supported.
func TxFromInternal(pl payload.Payload) (PlaintextTx, bool) {
    tx, ok := pl.(*plaintext.PlaintextTx)
    if !ok || tx.Type() != "plaintext_v1" {
        return PlaintextTx{}, false
    }
    return PlaintextTx{
        Type:  tx.Type(),
        From:  tx.From,
        Nonce: tx.Nonce,
        Gas:   tx.Gas,
        Fee:   tx.Fee,
        Sig:   tx.Sig,
    }, true
}

// ToInternal converts the wire tx back to a payload instance.
func (w PlaintextTx) ToInternal() payload.Payload {
    if w.Type != "plaintext_v1" {
        return nil
    }
    t := &plaintext.PlaintextTx{
        From:  w.From,
        Nonce: w.Nonce,
        Gas:   w.Gas,
        Fee:   w.Fee,
        Sig:   w.Sig,
    }
    return t
}

