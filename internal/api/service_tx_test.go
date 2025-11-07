package api

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    payload "github.com/zmlAEQ/Aequa-network/internal/payload"
    wire "github.com/zmlAEQ/Aequa-network/internal/p2p/wire"
)

// stubBroadcaster implements txBroadcaster for tests.
type stubBroadcaster struct{
    calls int
    last  payload.Payload
}

func (s *stubBroadcaster) BroadcastTx(_ context.Context, tx payload.Payload) error {
    s.calls++
    s.last = tx
    return nil
}

func TestHandleTxPlain_OK_PublishAndBroadcast(t *testing.T) {
    t.Setenv("AEQUA_ENABLE_TX_API", "1")
    s := &Service{addr: ":0", upstream: ""}
    // capture published payload
    published := 0
    var pub payload.Payload
    s.SetTxPublisher(func(_ context.Context, pl payload.Payload) { published++; pub = pl })
    // stub broadcaster
    sb := &stubBroadcaster{}
    s.SetTxBroadcaster(sb)

    wtx := wire.PlaintextTx{Type: "plaintext_v1", From: "A", Nonce: 0, Gas: 21000, Fee: 100, Sig: bytes.Repeat([]byte{1}, 32)}
    b, _ := json.Marshal(wtx)
    req := httptest.NewRequest(http.MethodPost, "/v1/tx/plain", bytes.NewReader(b))
    rr := httptest.NewRecorder()

    s.handleTxPlain(rr, req)

    if rr.Code != http.StatusAccepted {
        t.Fatalf("expected 202, got %d", rr.Code)
    }
    if published != 1 || pub == nil {
        t.Fatalf("expected published=1 with payload, got %d", published)
    }
    if sb.calls != 1 || sb.last == nil {
        t.Fatalf("expected broadcast=1 with payload, got %d", sb.calls)
    }
}

func TestHandleTxPlain_MethodNotAllowed(t *testing.T) {
    s := &Service{addr: ":0"}
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/v1/tx/plain", nil)
    s.handleTxPlain(rr, req)
    if rr.Code != http.StatusMethodNotAllowed {
        t.Fatalf("expected 405, got %d", rr.Code)
    }
}

func TestHandleTxPlain_InvalidJSON(t *testing.T) {
    s := &Service{addr: ":0"}
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/v1/tx/plain", bytes.NewBufferString("{"))
    s.handleTxPlain(rr, req)
    if rr.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rr.Code)
    }
}

func TestHandleTxPlain_InvalidTx(t *testing.T) {
    s := &Service{addr: ":0"}
    bad := wire.PlaintextTx{Type: "plaintext_v1", From: "", Nonce: 0, Gas: 0, Fee: 0, Sig: []byte{1,2,3}}
    b, _ := json.Marshal(bad)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/v1/tx/plain", bytes.NewReader(b))
    s.handleTxPlain(rr, req)
    if rr.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rr.Code)
    }
}

