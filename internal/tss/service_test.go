package tss

import (
    "context"
    "testing"
    "github.com/zmlAEQ/Aequa-network/internal/p2p"
)

func TestService_StartStop_NoLimiter(t *testing.T) {
    s := New(nil)
    if err := s.Start(context.Background()); err != nil { t.Fatalf("start: %v", err) }
    if err := s.Stop(context.Background()); err != nil { t.Fatalf("stop: %v", err) }
}

func TestService_StartStop_WithLimiter(t *testing.T) {
    p := p2p.New()
    // Configure limiter
    cfg := p2p.DefaultConfig(); cfg.MaxTSSSessions = 1
    p.SetConfig(cfg)
    s := New(p)
    if err := s.Start(context.Background()); err != nil { t.Fatalf("start: %v", err) }
    if err := s.Stop(context.Background()); err != nil { t.Fatalf("stop: %v", err) }
}