package consensus

import (
    "context"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/bus"
    "github.com/zmlAEQ/Aequa-network/pkg/lifecycle"
    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
    qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
    "github.com/zmlAEQ/Aequa-network/internal/state"
)

type Service struct{ sub bus.Subscriber; v qbft.Verifier; store state.Store; st qbft.Processor; wal *qbft.WAL }

func New() *Service { return &Service{} }
func NewWithSub(sub bus.Subscriber) *Service { return &Service{sub: sub} }
func (s *Service) Name() string { return "consensus" }

// SetVerifier allows tests/wiring to inject a qbft.Verifier. If nil, a BasicVerifier is instantiated on start.
func (s *Service) SetVerifier(v qbft.Verifier) { s.v = v }

// SetStore allows tests/wiring to inject a StateDB store. If nil, a MemoryStore is instantiated on start.
func (s *Service) SetStore(st state.Store) { s.store = st }

// SetProcessor allows tests/wiring to inject a qbft state processor. If nil, a default state is created on start.
func (s *Service) SetProcessor(p qbft.Processor) { s.st = p }

// SetWAL allows injecting a qbft WAL implementation (optional).
func (s *Service) SetWAL(w *qbft.WAL) { s.wal = w }

func (s *Service) Start(ctx context.Context) error {
    if s.sub == nil {
        logger.Info("consensus start (stub)")
        return nil
    }
    if s.v == nil { s.v = qbft.NewBasicVerifierWithPolicy(qbft.DefaultPolicy()) }
    if s.store == nil { s.store = state.NewMemoryStore() }
    if s.st == nil { s.st = &qbft.State{} }
    // Start E2E attack/testing endpoint when built with tag "e2e" (no-op otherwise).
    startE2E(s)
    if ls, err := s.store.LoadLastState(ctx); err != nil {
        logger.InfoJ("consensus_state", map[string]any{"op":"load", "result":"miss", "err": err.Error(), "trace_id": ""})
    } else {
        logger.InfoJ("consensus_state", map[string]any{"op":"load", "result":"ok", "height": ls.Height, "round": ls.Round, "trace_id": ""})
    }
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for {
            select {
            case ev := <-s.sub:
                // Count the event as received
                metrics.Inc("consensus_events_total", map[string]string{"kind": string(ev.Kind)})

                // Measure full processing time: verify -> state -> persist
                begin := time.Now()
                // Map event to qbft message via adapter
                msg := MapEventToQBFT(ev)
                if err := s.v.Verify(msg); err == nil {
                    // Persist vote intent to WAL before processing (best-effort)
                    if s.wal != nil && (msg.Type == qbft.MsgPrepare || msg.Type == qbft.MsgCommit) {
                        _ = s.wal.AppendIntent(msg)
                    }
                    _ = s.st.Process(msg)
                    if err2 := s.store.SaveLastState(ctx, state.LastState{Height: msg.Height, Round: msg.Round}); err2 != nil {
                        logger.ErrorJ("consensus_state", map[string]any{"op":"save", "result":"error", "err": err2.Error(), "trace_id": ev.TraceID})
                    } else {
                        logger.InfoJ("consensus_state", map[string]any{"op":"save", "result":"ok", "height": msg.Height, "round": msg.Round, "trace_id": ev.TraceID})
                    }
                }
                durMs := time.Since(begin).Milliseconds()
                // Audit log and summary with the full processing latency; labels unchanged
                logger.InfoJ("consensus_recv", map[string]any{"kind": string(ev.Kind), "trace_id": ev.TraceID, "result": "recv", "latency_ms": durMs})
                metrics.ObserveSummary("consensus_proc_ms", map[string]string{"kind": string(ev.Kind)}, float64(durMs))
            case <-ctx.Done():
                return
            }
        }
    }()
    return nil
}

func (s *Service) Stop(ctx context.Context) error  { logger.Info("consensus stop (stub)"); return nil }

var _ lifecycle.Service = (*Service)(nil)


