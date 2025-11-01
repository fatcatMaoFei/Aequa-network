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
    // 设置 TSS 并发上限为 1
    cfg := DefaultConfig()
    cfg.MaxTSSSessions = 1
    s.SetConfig(cfg)
    if err := s.Start(context.Background()); err != nil { t.Fatalf("start: %v", err) }
    defer s.Stop(context.Background())

    if !s.TryOpenTSS() { t.Fatalf("first TryOpenTSS should allow") }
    if s.TryOpenTSS() { t.Fatalf("second TryOpenTSS should be limited") }
    dump := metrics.DumpProm()
    if !strings.Contains(dump, `tss_rate_limited_total{kind="tss_session"} 1`) {
        t.Fatalf("want rate limited metric, got: %s", dump)
    }
    // 关闭一个会话，稍等指标异步无关但我们直接继续
    s.CloseTSS()
    // 尝试再次打开，应当允许
    if !s.TryOpenTSS() { t.Fatalf("after close, TryOpenTSS should allow again") }
    // 检查 gauge 至少存在
    time.Sleep(10 * time.Millisecond)
    dump = metrics.DumpProm()
    if !strings.Contains(dump, `tss_sessions_open`) {
        t.Fatalf("missing tss_sessions_open gauge: %s", dump)
    }
}

