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
    // Optional SessionStore for resume/retry.
    Sess *SessionStore
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

func (e *EngineImpl) snapshot(s *session) sessionState {
    toSlice := func(m map[string]struct{}) []string {
        arr := make([]string, 0, len(m))
        for k := range m { arr = append(arr, k) }
        return arr
    }
    return sessionState{Epoch: s.epoch, Propose: toSlice(s.propose), Commit: toSlice(s.commit), Reveal: toSlice(s.reveal), Ack: toSlice(s.ack), Done: s.done}
}

func (e *EngineImpl) restore(st sessionState) *session {
    toMap := func(a []string) map[string]struct{} {
        m := map[string]struct{}{}
        for _, k := range a { m[k] = struct{}{} }
        return m
    }
    return &session{epoch: st.Epoch, propose: toMap(st.Propose), commit: toMap(st.Commit), reveal: toMap(st.Reveal), ack: toMap(st.Ack), done: st.Done}
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
        // Complaint triggers epoch bump and phase reset (retry)
        s.epoch++
        s.propose = map[string]struct{}{}
        s.commit = map[string]struct{}{}
        s.reveal = map[string]struct{}{}
        s.ack = map[string]struct{}{}
    default:
        // Unknown types ignored
    }
    // persist session snapshot if store provided
    if e.cfg.Sess != nil {
        _ = e.cfg.Sess.Save(msg.SessionID, e.snapshot(s))
    }
    e.mu.Unlock()
    return advanced, nil
}

// Resume loads a session state from store (if provided) into memory.
func (e *EngineImpl) Resume(id string) error {
    if e.cfg.Sess == nil { return ErrSessNotFound }
    st, err := e.cfg.Sess.Load(id)
    if err != nil { return err }
    e.mu.Lock()
    e.sess[id] = e.restore(st)
    e.mu.Unlock()
    return nil
}

