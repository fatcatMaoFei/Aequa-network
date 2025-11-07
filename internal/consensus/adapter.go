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
    // Fallback: map generic events to a placeholder prepare vote for visibility.
    id := fmt.Sprintf("ev-%s-%d-%d", ev.TraceID, ev.Height, ev.Round)
    return qbft.Message{
        ID:      id,
        From:    "consensus_stub",
        Type:    qbft.MsgPrepare,
        Height:  ev.Height,
        Round:   ev.Round,
        Payload: nil,
        TraceID: ev.TraceID,
        Sig:     nil,
    }
}

