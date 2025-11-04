package tss

import (
    "context"
    "time"

    "github.com/zmlAEQ/Aequa-network/internal/p2p"
    "github.com/zmlAEQ/Aequa-network/pkg/lifecycle"
    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

type Service struct{ p2p *p2p.Service; opened bool }

func New(p2pSvc *p2p.Service) *Service { return &Service{p2p: p2pSvc} }

func (s *Service) Name() string { return "tss" }

func (s *Service) Start(ctx context.Context) error {
    begin := time.Now()
    // Try to open a TSS session slot if limiter configured; otherwise it's a no-op.
    if s.p2p != nil && s.p2p.TryOpenTSS() {
        s.opened = true
    }
    dur := time.Since(begin).Milliseconds()
    logger.InfoJ("service_op", map[string]any{"service":"tss", "op":"start", "result":"ok", "latency_ms": dur})
    metrics.ObserveSummary("service_op_ms", map[string]string{"service":"tss", "op":"start"}, float64(dur))
    return nil
}

func (s *Service) Stop(ctx context.Context) error {
    begin := time.Now()
    if s.opened && s.p2p != nil { s.p2p.CloseTSS() }
    dur := time.Since(begin).Milliseconds()
    logger.InfoJ("service_op", map[string]any{"service":"tss", "op":"stop", "result":"ok", "latency_ms": dur})
    metrics.ObserveSummary("service_op_ms", map[string]string{"service":"tss", "op":"stop"}, float64(dur))
    return nil
}

var _ lifecycle.Service = (*Service)(nil)