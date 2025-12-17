package consensus

import (
	"testing"

	qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
	"github.com/zmlAEQ/Aequa-network/pkg/bus"
)

func TestMapEventToQBFT_BasicMapping(t *testing.T) {
	ev := bus.Event{Kind: bus.KindDuty, Height: 42, Round: 3, TraceID: "tid123"}
	msg := MapEventToQBFT(ev)

	if msg.Type != qbft.MsgPreprepare {
		t.Fatalf("type: got %q", msg.Type)
	}
	if msg.From != "consensus_stub" {
		t.Fatalf("from: got %q", msg.From)
	}
	if msg.Height != 42 || msg.Round != 0 {
		t.Fatalf("coords: got (%d,%d)", msg.Height, msg.Round)
	}
	if msg.TraceID != "tid123" {
		t.Fatalf("trace: got %q", msg.TraceID)
	}
	wantID := "ev-tid123-42-0"
	if msg.ID != wantID {
		t.Fatalf("id: got %q want %q", msg.ID, wantID)
	}
	if len(msg.Sig) != 0 {
		t.Fatalf("sig: expected empty")
	}
	if msg.Payload != nil {
		t.Fatalf("payload: expected nil stub")
	}
}
