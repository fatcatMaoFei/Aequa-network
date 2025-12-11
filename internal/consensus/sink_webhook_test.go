package consensus

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookSink_Publish_OK(t *testing.T) {
	var got int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ws := WebhookSink{URL: srv.URL, Timeout: 200 * time.Millisecond}
	ws.Publish(ValueRecord{Height: 1, Bids: 10, Fees: 5, Items: 2})
	if got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestWebhookSink_Publish_BadURL(t *testing.T) {
	ws := WebhookSink{URL: "://bad"}
	// Should not panic
	ws.Publish(ValueRecord{})
}
