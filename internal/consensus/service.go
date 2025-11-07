package consensus

import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/bus"
    "github.com/zmlAEQ/Aequa-network/pkg/lifecycle"
    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
    qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
    pl "github.com/zmlAEQ/Aequa-network/internal/payload"
    "github.com/zmlAEQ/Aequa-network/internal/state"
    "os"
)

// QbftBroadcaster is a minimal broadcaster abstraction to decouple P2P transport.
// Any type implementing BroadcastQBFT(ctx,msg) can be injected (e.g., libp2p transport).
type QbftBroadcaster interface{ BroadcastQBFT(ctx context.Context, msg qbft.Message) error }

type Service struct{ sub bus.Subscriber; v qbft.Verifier; store state.Store; st qbft.Processor; wal *qbft.WAL; lastWAL qbft.Message; tss TSSAggVerifier; enableTSSSync bool; enableBuilder bool; pool *pl.Container; policy pl.BuilderPolicy; lastBlock map[uint64]map[uint64]pl.StandardBlock; enableTSSSign bool; signer TSSSigner; bc QbftBroadcaster }

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

// TSSSigner is a minimal signer interface to avoid importing the TSS API here.
type TSSSigner interface{ Sign(ctx context.Context, height, round uint64, msg []byte) ([]byte, error) }

// SetPayloadContainer wires a pluggable mempool container into consensus (optional).
func (s *Service) SetPayloadContainer(c *pl.Container) { s.pool = c }

// SetBuilderPolicy defines deterministic selection order/limits.
func (s *Service) SetBuilderPolicy(p pl.BuilderPolicy) { s.policy = p }

// SetTSSSigner injects an optional TSS signer used at commit when enabled.
func (s *Service) SetTSSSigner(si TSSSigner) { s.signer = si }

// SetBroadcaster injects an optional broadcaster used to publish local votes
// (prepare/commit) to the network. When nil, broadcasting is disabled.
func (s *Service) SetBroadcaster(b QbftBroadcaster) { s.bc = b }

func (s *Service) Start(ctx context.Context) error {
    if s.sub == nil {
        logger.Info("consensus start (stub)")
        return nil
    }
    if s.v == nil { s.v = qbft.NewBasicVerifierWithPolicy(qbft.DefaultPolicy()) }
    if s.store == nil { s.store = state.NewMemoryStore() }
    if s.st == nil { s.st = &qbft.State{} }
    s.enableTSSSync = os.Getenv("AEQUA_ENABLE_TSS_STATE_SYNC") == "1"
    s.enableBuilder = os.Getenv("AEQUA_ENABLE_BUILDER") == "1"
    s.enableTSSSign = os.Getenv("AEQUA_ENABLE_TSS_SIGN") == "1"
    if s.lastBlock == nil { s.lastBlock = make(map[uint64]map[uint64]pl.StandardBlock) }
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
                // Handle transaction gossip (if any) before consensus mapping.
                if ev.Kind == bus.KindTx {
                    if s.pool != nil {
                        if plAny, ok := ev.Body.(pl.Payload); ok && plAny != nil {
                            _ = s.pool.Add(plAny)
                        }
                    }
                    // Proceed to next event; tx does not map to qbft.
                    continue
                }
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
                    // Optional: broadcast local votes (prepare/commit) via injected broadcaster.
                    if s.bc != nil && (msg.Type == qbft.MsgPrepare || msg.Type == qbft.MsgCommit) {
                        if err := s.bc.BroadcastQBFT(ctx, msg); err != nil {
                            metrics.Inc("consensus_broadcast_total", map[string]string{"type": string(msg.Type), "result":"error"})
                            logger.ErrorJ("consensus_broadcast", map[string]any{"result":"error", "type": string(msg.Type), "height": msg.Height, "round": msg.Round, "trace_id": ev.TraceID, "err": err.Error()})
                        } else {
                            metrics.Inc("consensus_broadcast_total", map[string]string{"type": string(msg.Type), "result":"ok"})
                            logger.InfoJ("consensus_broadcast", map[string]any{"result":"ok", "type": string(msg.Type), "height": msg.Height, "round": msg.Round, "trace_id": ev.TraceID})
                        }
                    }
                    // Behind-flag builder: prepare deterministic block for this coordinate
                    if s.enableBuilder && s.pool != nil {
                        hdr := pl.BlockHeader{Height: msg.Height, Round: msg.Round}
                        blk := pl.PrepareProposal(s.pool, hdr, s.policy)
                        if err := pl.ProcessProposal(blk, s.policy); err == nil {
                            if s.lastBlock[msg.Height] == nil { s.lastBlock[msg.Height] = make(map[uint64]pl.StandardBlock) }
                            s.lastBlock[msg.Height][msg.Round] = blk
                            logger.InfoJ("consensus_builder", map[string]any{"result":"ok", "height": msg.Height, "round": msg.Round, "items": len(blk.Items)})
                        } else {
                            logger.ErrorJ("consensus_builder", map[string]any{"result":"reject", "err": err.Error(), "height": msg.Height, "round": msg.Round})
                        }
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
                        if s.enableTSSSign && s.signer != nil && msg.Type == qbft.MsgCommit {
                            if row, ok := s.lastBlock[msg.Height]; ok {
                                if blk, ok2 := row[msg.Round]; ok2 {
                                    b, _ := json.Marshal(blk)
                                    sum := sha256.Sum256(b)
                                    if _, err := s.signer.Sign(ctx, msg.Height, msg.Round, sum[:]); err != nil {
                                        metrics.Inc("block_sign_total", map[string]string{"result":"error"})
                                        logger.ErrorJ("consensus_block", map[string]any{"op":"sign", "result":"error", "err": err.Error(), "height": msg.Height, "round": msg.Round})
                                    } else {
                                        metrics.Inc("block_sign_total", map[string]string{"result":"ok"})
                                        logger.InfoJ("consensus_block", map[string]any{"op":"sign", "result":"ok", "height": msg.Height, "round": msg.Round})
                                    }
                                }
                            }
                        }
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


