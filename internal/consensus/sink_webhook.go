package consensus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/zmlAEQ/Aequa-network/pkg/logger"
)

// WebhookSink posts ValueRecord to a configured endpoint; best-effort.
type WebhookSink struct {
	URL     string
	Timeout time.Duration
}

func (w WebhookSink) Publish(v ValueRecord) {
	if w.URL == "" {
		return
	}
	payload, err := json.Marshal(v)
	if err != nil {
		logger.ErrorJ("fee_sink", map[string]any{"result": "marshal_error", "err": err.Error()})
		return
	}
	client := &http.Client{Timeout: w.timeout()}
	req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(payload))
	if err != nil {
		logger.ErrorJ("fee_sink", map[string]any{"result": "request_error", "err": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.ErrorJ("fee_sink", map[string]any{"result": "post_error", "err": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		logger.ErrorJ("fee_sink", map[string]any{"result": "remote_error", "code": resp.StatusCode})
		return
	}
	logger.InfoJ("fee_sink", map[string]any{"result": "ok", "code": resp.StatusCode})
}

func (w WebhookSink) timeout() time.Duration {
	if w.Timeout > 0 {
		return w.Timeout
	}
	return 500 * time.Millisecond
}
