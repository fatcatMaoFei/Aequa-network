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
    "os"
)

type Service struct{ sub bus.Subscriber; v qbft.Verifier; store state.Store; st qbft.Processor; wal *qbft.WAL; lastWAL qbft.Message; tss TSSAggVerifier; enableTSSSync bool }

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

// TSSAggVerifier provides the minimal aggregate signature verification API.
// It matches the needed method from internal/tss/api without importing it here.
type TSSAggVerifier interface{ VerifyAgg(pkGroup []byte, msg []byte, aggSig []byte) bool }

// SetTSSVerifier injects a TSS aggregate signature verifier (optional).
func (s *Service) SetTSSVerifier(v TSSAggVerifier) { s.tss = v }

func (s *Service) Start(ctx context.Context) error {
    if s.sub == nil {
        logger.Info("consensus start (stub)")
        return nil
    }
    if s.v == nil { s.v = qbft.NewBasicVerifierWithPolicy(qbft.DefaultPolicy()) }
    if s.store == nil { s.store = state.NewMemoryStore() }
    if s.st == nil { s.st = &qbft.State{} }
    s.enableTSSSync = os.Getenv("AEQUA_ENABLE_TSS_STATE_SYNC") == "1"
    // Start E2E attack/testing endpoint when built with tag "e2e" (no-op otherwise).
    startE2E(s)
    // Try recover last vote intent from WAL (best-effort guard)
    if s.wal != nil {
        if last, err := s.wal.LastIntent(); err == nil {
            s.lastWAL = last
            logger.InfoJ("qbft_wal_guard", map[string]any{"result":"ok", "height": last.Height, "round": last.Round})
        } else {
            logger.InfoJ("qbft_wal_guard", map[string]any{"result":"miss"})
        }
    }
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
                    // Guard against processing intents older than last WAL entry (best-effort)
                    allowed := true
                    if (msg.Type == qbft.MsgPrepare || msg.Type == qbft.MsgCommit) && s.lastWAL.Type != "" {
                        if msg.Height < s.lastWAL.Height || (msg.Height == s.lastWAL.Height && msg.Round < s.lastWAL.Round) {
                            allowed = false
                            metrics.Inc("qbft_wal_guard_drops_total", nil)
                            logger.InfoJ("qbft_wal_guard", map[string]any{"result":"drop", "height": msg.Height, "round": msg.Round, "last_h": s.lastWAL.Height, "last_r": s.lastWAL.Round})
                        }
                    }
                    if allowed {
                        _ = s.st.Process(msg)
                        if err2 := s.store.SaveLastState(ctx, state.LastState{Height: msg.Height, Round: msg.Round}); err2 != nil {
                            logger.ErrorJ("consensus_state", map[string]any{"op":"save", "result":"error", "err": err2.Error(), "trace_id": ev.TraceID})
                        } else {
                            logger.InfoJ("consensus_state", map[string]any{"op":"save", "result":"ok", "height": msg.Height, "round": msg.Round, "trace_id": ev.TraceID})
                        }
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

// VerifyHeaderWithTSS verifies a header blob and aggregate signature under the
// provided group public key via the injected TSS verifier. It is behind a soft
// enable flag (AEQUA_ENABLE_TSS_STATE_SYNC). Metrics family is additive.
func (s *Service) VerifyHeaderWithTSS(pkGroup, header, sig []byte) bool {
    if !s.enableTSSSync || s.tss == nil {
        metrics.Inc("state_sync_verified_total", map[string]string{"result":"disabled"})
        logger.InfoJ("consensus_state_sync", map[string]any{"result":"disabled"})
        return false
    }
    ok := s.tss.VerifyAgg(pkGroup, header, sig)
    if ok {
        metrics.Inc("state_sync_verified_total", map[string]string{"result":"ok"})
        logger.InfoJ("consensus_state_sync", map[string]any{"result":"ok"})
    } else {
        metrics.Inc("state_sync_verified_total", map[string]string{"result":"fail"})
        logger.ErrorJ("consensus_state_sync", map[string]any{"result":"fail"})
    }
    return ok
}


