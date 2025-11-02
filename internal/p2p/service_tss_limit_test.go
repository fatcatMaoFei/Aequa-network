package p2p

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func TestService_TSSLimiter_AllowsThenLimitsThenAllows(t *testing.T) {
    metrics.Reset()
    s := New()
    cfg := DefaultConfig()
    cfg.MaxTSSSessions = 1
    s.SetConfig(cfg)
    if err := s.Start(context.Background()); err != nil {
        t.Fatalf("start: %v", err)
    }
    defer func() { _ = s.Stop(context.Background()) }()

    if !s.TryOpenTSS() {
        t.Fatalf("first TryOpenTSS should allow")
    }
    if s.TryOpenTSS() {
        t.Fatalf("second TryOpenTSS should be limited")
    }
    dump := metrics.DumpProm()
    if !strings.Contains(dump, `tss_rate_limited_total{kind="tss_session"} 1`) {
        t.Fatalf("want rate limited metric, got: %s", dump)
    }
    s.CloseTSS()

    // small wait to allow gauge update path to run
    time.Sleep(10 * time.Millisecond)

    if !s.TryOpenTSS() {
        t.Fatalf("after close, TryOpenTSS should allow again")
    }
    dump = metrics.DumpProm()
    if !strings.Contains(dump, "tss_sessions_open") {
        t.Fatalf("missing tss_sessions_open gauge: %s", dump)
    }
}