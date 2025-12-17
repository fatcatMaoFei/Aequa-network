package consensus

import (
	"fmt"

	qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
	"github.com/zmlAEQ/Aequa-network/pkg/bus"
)

// MapEventToQBFT converts a bus.Event into a qbft.Message.
// Stub mapping: direct field mapping with conservative defaults.
func MapEventToQBFT(ev bus.Event) qbft.Message {
	// Preferred: when P2P delivers a real qbft.Message via the bus, consume it.
	if ev.Kind == bus.KindConsensus {
		if msg, ok := ev.Body.(qbft.Message); ok {
			return msg
		}
	}
	// Fallback: map duty to a valid preprepare (round=0) so the state machine can advance.
	// Round is forced to 0 to satisfy the current verifier semantic.
	round := uint64(0)
	id := fmt.Sprintf("ev-%s-%d-%d", ev.TraceID, ev.Height, round)
	return qbft.Message{
		ID:      id,
		From:    "consensus_stub",
		Type:    qbft.MsgPreprepare,
		Height:  ev.Height,
		Round:   round,
		Payload: nil,
		TraceID: ev.TraceID,
		Sig:     nil,
	}
}
