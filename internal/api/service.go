package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	wire "github.com/zmlAEQ/Aequa-network/internal/p2p/wire"
	payload "github.com/zmlAEQ/Aequa-network/internal/payload"
	"github.com/zmlAEQ/Aequa-network/pkg/lifecycle"
	"github.com/zmlAEQ/Aequa-network/pkg/logger"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
	"github.com/zmlAEQ/Aequa-network/pkg/trace"
	"os"
)

type txBroadcaster interface {
	BroadcastTx(ctx context.Context, tx payload.Payload) error
}

type Service struct {
	addr        string
	srv         *http.Server
	onPublish   func(ctx context.Context, payload []byte) error
	upstream    string
	txb         txBroadcaster
	onPublishTx func(ctx context.Context, pl payload.Payload)
}

func New(addr string, onPublish func(ctx context.Context, payload []byte) error, upstream string) *Service {
	return &Service{addr: addr, onPublish: onPublish, upstream: upstream}
}

func (s *Service) Name() string { return "api" }

func (s *Service) Start(ctx context.Context) error {
	begin := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/duty", s.handleDuty)
	if os.Getenv("AEQUA_ENABLE_TX_API") == "1" {
		mux.HandleFunc("/v1/tx/plain", s.handleTxPlain)
	}
	mux.HandleFunc("/", s.proxy)
	s.srv = &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		logger.Info(fmt.Sprintf("api listening on %s\n", s.addr))
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server error: " + err.Error())
		}
	}()
	dur := time.Since(begin).Milliseconds()
	logger.InfoJ("service_op", map[string]any{"service": "api", "op": "start", "result": "ok", "latency_ms": dur})
	metrics.ObserveSummary("service_op_ms", map[string]string{"service": "api", "op": "start"}, float64(dur))
	return nil
}
func (s *Service) Stop(ctx context.Context) error {
	begin := time.Now()
	if s.srv == nil {
		dur := time.Since(begin).Milliseconds()
		logger.InfoJ("service_op", map[string]any{"service": "api", "op": "stop", "result": "ok", "latency_ms": dur})
		metrics.ObserveSummary("service_op_ms", map[string]string{"service": "api", "op": "stop"}, float64(dur))
		return nil
	}
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := s.srv.Shutdown(ctx2)
	dur := time.Since(begin).Milliseconds()
	if err != nil {
		logger.ErrorJ("service_op", map[string]any{"service": "api", "op": "stop", "result": "error", "err": err.Error(), "latency_ms": dur})
	} else {
		logger.InfoJ("service_op", map[string]any{"service": "api", "op": "stop", "result": "ok", "latency_ms": dur})
	}
	metrics.ObserveSummary("service_op_ms", map[string]string{"service": "api", "op": "stop"}, float64(dur))
	return err
}

var _ lifecycle.Service = (*Service)(nil)

// handleDuty accepts a JSON body and publishes it after basic validation.
func (s *Service) handleDuty(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tid := traceID(r)
	route := "/v1/duty"

	if r.Method != http.MethodPost {
		s.logAPI(w, route, http.StatusMethodNotAllowed, start, tid, "error", "method not allowed")
		return
	}
	if r.Body == nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", "empty body")
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", "read error")
		return
	}
	if err = validateDutyJSON(b); err != nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", err.Error())
		return
	}

	if s.onPublish != nil {
		_ = s.onPublish(trace.WithTraceID(r.Context(), tid), b)
	}
	dur := time.Since(start)
	metrics.Inc("api_requests_total", map[string]string{"route": route, "code": "202"})
	metrics.ObserveSummary("api_latency_ms", map[string]string{"route": route}, float64(dur.Milliseconds()))
	logger.InfoJ("api_request", map[string]any{
		"route":      route,
		"code":       202,
		"bytes":      len(b),
		"latency_ms": dur.Milliseconds(),
		"result":     "accepted",
		"trace_id":   tid,
	})
	w.WriteHeader(http.StatusAccepted)
}

type dutyEnvelope struct {
	Type    string `json:"type"`
	Height  uint64 `json:"height"`
	Round   uint64 `json:"round"`
	Payload any    `json:"payload"`
}

func validateDutyJSON(b []byte) error {
	var d dutyEnvelope
	if len(b) == 0 {
		return fmt.Errorf("empty")
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("too large")
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return fmt.Errorf("invalid json")
	}
	switch d.Type {
	case "attester", "proposer", "sync":
	default:
		return fmt.Errorf("invalid type")
	}
	if d.Height > 1<<62 {
		return fmt.Errorf("height out of range")
	}
	if d.Round > 1<<40 {
		return fmt.Errorf("round out of range")
	}
	return nil
}

func (s *Service) proxy(w http.ResponseWriter, r *http.Request) {
	if s.upstream == "" {
		http.NotFound(w, r)
		return
	}
	u, err := url.Parse(s.upstream)
	if err != nil {
		http.Error(w, "bad upstream", http.StatusBadGateway)
		return
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	start := time.Now()
	rp.ModifyResponse = func(resp *http.Response) error {
		metrics.Inc("api_requests_total", map[string]string{"route": "proxy", "code": fmt.Sprintf("%d", resp.StatusCode)})
		metrics.ObserveSummary("api_latency_ms", map[string]string{"route": "proxy"}, float64(time.Since(start).Milliseconds()))
		logger.InfoJ("api_request", map[string]any{
			"route":      "proxy",
			"code":       resp.StatusCode,
			"latency_ms": time.Since(start).Milliseconds(),
			"result":     "ok",
			"trace_id":   traceID(r),
		})
		return nil
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		metrics.Inc("api_requests_total", map[string]string{"route": "proxy", "code": "502"})
		metrics.ObserveSummary("api_latency_ms", map[string]string{"route": "proxy"}, float64(time.Since(start).Milliseconds()))
		logger.ErrorJ("api_request", map[string]any{
			"route":    "proxy",
			"code":     502,
			"err":      e.Error(),
			"result":   "error",
			"trace_id": traceID(r),
		})
		http.Error(w, "upstream error", http.StatusBadGateway)
	}
	rp.ServeHTTP(w, r)
}

// traceID returns request trace id from header or generates a simple one.
func traceID(r *http.Request) string {
	if t := r.Header.Get("X-Trace-ID"); t != "" {
		return t
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// logAPI records unified metrics and logs, then writes an error response body.
func (s *Service) logAPI(w http.ResponseWriter, route string, code int, start time.Time, tid, result, errMsg string) {
	dur := time.Since(start)
	metrics.Inc("api_requests_total", map[string]string{"route": route, "code": fmt.Sprintf("%d", code)})
	metrics.ObserveSummary("api_latency_ms", map[string]string{"route": route}, float64(dur.Milliseconds()))
	fields := map[string]any{
		"route":      route,
		"code":       code,
		"latency_ms": dur.Milliseconds(),
		"result":     result,
		"trace_id":   tid,
	}
	if errMsg != "" {
		fields["err"] = errMsg
	}
	if code >= 400 {
		logger.ErrorJ("api_request", fields)
	} else {
		logger.InfoJ("api_request", fields)
	}
	body := errMsg
	if body == "" {
		body = http.StatusText(code)
	}
	http.Error(w, body, code)
}

// handleHealth returns ok and records unified logs and metrics.
func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
	dur := time.Since(start)
	metrics.Inc("api_requests_total", map[string]string{"route": "/health", "code": "200"})
	metrics.ObserveSummary("api_latency_ms", map[string]string{"route": "/health"}, float64(dur.Milliseconds()))
	logger.InfoJ("api_request", map[string]any{
		"route":      "/health",
		"code":       200,
		"latency_ms": dur.Milliseconds(),
		"result":     "ok",
		"trace_id":   traceID(r),
	})
}

// SetTxBroadcaster injects an optional P2P broadcaster for tx gossip (behind flag).
func (s *Service) SetTxBroadcaster(b txBroadcaster) { s.txb = b }

// handleTxPlain accepts a plaintext_v1 transaction and optionally gossips it.
func (s *Service) handleTxPlain(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tid := traceID(r)
	route := "/v1/tx/plain"
	if r.Method != http.MethodPost {
		s.logAPI(w, route, http.StatusMethodNotAllowed, start, tid, "error", "method not allowed")
		return
	}
	if r.Body == nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", "empty body")
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", "read error")
		return
	}
	pl, err := wire.ParseTx(b)
	if err != nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", "invalid json")
		return
	}
	// Guard private_v1 behind explicit flag.
	if pl != nil && pl.Type() == "private_v1" && os.Getenv("AEQUA_ENABLE_BEAST") != "1" {
		s.logAPI(w, route, http.StatusServiceUnavailable, start, tid, "error", "beast disabled")
		return
	}
	if pl == nil || pl.Validate() != nil {
		s.logAPI(w, route, http.StatusBadRequest, start, tid, "error", "invalid tx")
		return
	}
	// Publish to bus for local mempool ingest (structured payload)
	if s.onPublishTx != nil {
		s.onPublishTx(trace.WithTraceID(r.Context(), tid), pl)
	}
	// Optionally broadcast via P2P
	if s.txb != nil {
		_ = s.txb.BroadcastTx(r.Context(), pl)
	}
	dur := time.Since(start)
	metrics.Inc("api_requests_total", map[string]string{"route": route, "code": "202"})
	metrics.ObserveSummary("api_latency_ms", map[string]string{"route": route}, float64(dur.Milliseconds()))
	logger.InfoJ("api_request", map[string]any{"route": route, "code": 202, "bytes": len(b), "latency_ms": dur.Milliseconds(), "result": "accepted", "trace_id": tid})
	w.WriteHeader(http.StatusAccepted)
}

// SetTxPublisher allows wiring a callback to publish validated txs to the bus.
func (s *Service) SetTxPublisher(fn func(ctx context.Context, pl payload.Payload)) {
	s.onPublishTx = fn
}
