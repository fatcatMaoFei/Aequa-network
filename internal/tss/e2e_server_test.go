//go:build e2e

package tss

import (
    "io"
    "net/http"
    "testing"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func TestE2E_TSS_ServerRoutes(t *testing.T) {
    // Start e2e server on a fixed local port for test
    srv := StartE2E("127.0.0.1:4611", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    }))
    if srv == nil {
        t.Fatalf("server should be non-nil in e2e build")
    }
    defer srv.Close()

    // Wait a moment for server to start
    time.Sleep(30 * time.Millisecond)

    // /health
    resp, err := http.Get("http://127.0.0.1:4611/health")
    if err != nil { t.Fatalf("health: %v", err) }
    resp.Body.Close()
    if resp.StatusCode != http.StatusOK { t.Fatalf("health status=%d", resp.StatusCode) }

    // /e2e/tss
    resp2, err := http.Get("http://127.0.0.1:4611/e2e/tss")
    if err != nil { t.Fatalf("e2e: %v", err) }
    _, _ = io.ReadAll(resp2.Body)
    resp2.Body.Close()
    if resp2.StatusCode != http.StatusOK { t.Fatalf("e2e status=%d", resp2.StatusCode) }

    dump := metrics.DumpProm()
    if !contains(dump, `tss_e2e_requests_total{code="200"}`) {
        t.Fatalf("missing e2e request metric: %s", dump)
    }
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (contains(s[1:], sub) || s[:len(sub)] == sub))) }