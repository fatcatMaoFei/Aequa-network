//go:build e2e

package tss

import (
    "strconv"
    "net/http"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// StartE2E launches a minimal HTTP server for TSS testing under e2e builds.
// The provided handler is mounted at /e2e/tss; a /health endpoint returns 200.
// Typical usage in tests:
//   srv := tss.StartE2E(":4611", http.HandlerFunc(myHandler))
//   defer srv.Close()
func StartE2E(addr string, h http.Handler) *http.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
    mux.Handle("/e2e/tss", wrapMetrics(h))
    srv := &http.Server{Addr: addr, Handler: mux}
    go func() { _ = srv.ListenAndServe() }()
    return srv
}

func wrapMetrics(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rr := &respRec{ResponseWriter: w, code: 200}
        h.ServeHTTP(rr, r)
        metrics.Inc("tss_e2e_requests_total", map[string]string{"code": itoa(rr.code)})
        metrics.ObserveSummary("tss_e2e_latency_ms", nil, float64(time.Since(start).Milliseconds()))
        logger.InfoJ("tss_attack", map[string]any{"code": rr.code, "latency_ms": time.Since(start).Milliseconds(), "trace_id": r.Header.Get("X-Trace-ID")})
    })
}

type respRec struct{ http.ResponseWriter; code int }
func (r *respRec) WriteHeader(c int) { r.code = c; r.ResponseWriter.WriteHeader(c) }

// Small local integer to string to avoid fmt import.
func itoa(i int) string { return strconv.Itoa(i) }

