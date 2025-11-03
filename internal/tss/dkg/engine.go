package dkg

import (
    "context"
    "sync"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// Config controls the minimal DKG engine behaviour.
type Config struct {
    // N is total participants; T is threshold. Only T is used for phase advance in this MVP.
    N int
    T int
    // Optional KeyStore for persisting a placeholder KeyShare on finalize.
    Store *KeyStore
}

// EngineImpl provides a minimal in-memory DKG state machine to exercise a closed loop
// (Propose -> Commit -> Reveal -> Ack), with dedup per phase and complaint handling.
type EngineImpl struct {
    mu   sync.Mutex
    cfg  Config
    sess map[string]*session
}

type session struct {
    epoch   uint64
    propose map[string]struct{}
    commit  map[string]struct{}
    reveal  map[string]struct{}
    ack     map[string]struct{}
    done    bool
}

// NewEngine constructs a new minimal DKG engine.
func NewEngine(cfg Config) *EngineImpl {
    if cfg.T <= 0 { cfg.T = 2 }
    return &EngineImpl{cfg: cfg, sess: make(map[string]*session)}
}

// OnMessage implements Engine. It returns true when the session transitions to done.
func (e *EngineImpl) OnMessage(msg Message) (bool, error) {
    // Count incoming messages by type for observability (new family)
    metrics.Inc("tss_msgs_total", map[string]string{"type": string(msg.Type)})

    e.mu.Lock()
    s := e.sess[msg.SessionID]
    if s == nil {
        s = &session{epoch: msg.Epoch, propose: map[string]struct{}{}, commit: map[string]struct{}{}, reveal: map[string]struct{}{}, ack: map[string]struct{}{}}
        e.sess[msg.SessionID] = s
    }
    // Basic epoch monotonicity guard: ignore lower epoch
    if msg.Epoch < s.epoch { e.mu.Unlock(); return false, nil }
    // Accept monotonic epoch updates
    if msg.Epoch > s.epoch { s.epoch = msg.Epoch }

    advanced := false
    switch msg.Type {
    case MsgPropose:
        s.propose[msg.From] = struct{}{}
    case MsgCommit:
        s.commit[msg.From] = struct{}{}
    case MsgReveal:
        s.reveal[msg.From] = struct{}{}
    case MsgAck:
        s.ack[msg.From] = struct{}{}
        // Finalize when ack reaches threshold
        if !s.done && len(s.ack) >= e.cfg.T {
            s.done = true
            advanced = true
            // Persist a placeholder KeyShare to exercise atomic store (optional)
            if e.cfg.Store != nil {
                _ = e.cfg.Store.SaveKeyShare(context.Background(), KeyShare{Index: 0})
            }
            metrics.Inc("tss_sessions_total", map[string]string{"result": "ok"})
        }
    case MsgComplaint:
        // No state advance; could record a metric for complaints via tss_msgs_total above
        // Keep as no-op transition
    default:
        // Unknown types ignored
    }
    e.mu.Unlock()
    return advanced, nil
}

