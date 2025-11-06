//go:build e2e

package tss

import (
    "io"
    "net/http"
    "testing"
    "time"
)

func TestE2EServer_HealthAndHandler(t *testing.T) {
    h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
    srv := StartE2E(":4611", h)
    t.Cleanup(func(){ _ = srv.Close() })

    // wait a bit for listener
    time.Sleep(100 * time.Millisecond)

    // health
    if resp, err := http.Get("http://127.0.0.1:4611/health"); err != nil || resp.StatusCode != 200 {
        t.Fatalf("health: %v code=%v", err, resp.StatusCode)
    }

    // handler
    req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:4611/e2e/tss", nil)
    req.Header.Set("X-Trace-ID", "t")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { t.Fatalf("handler: %v", err) }
    _, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != http.StatusNoContent { t.Fatalf("want 204 got %d", resp.StatusCode) }
}

