package dkg

import (
    "context"
    "sync"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// Config controls the minimal DKG engine behaviour.
type Config struct {\n    // N is total participants; T is threshold. Only T is used for phase advance in this MVP.\n    N int\n    T int\n    // Optional KeyStore for persisting a placeholder KeyShare on finalize.\n    Store *KeyStore\n    // Optional SessionStore for resume/retry.\n    Sess  *SessionStore\n}

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
    toSlice := func(m map[string]struct{}) []string { arr:=make([]string,0,len(m)); for k := range m { arr = append(arr,k) }; return arr }
    return sessionState{Epoch:s.epoch, Propose:toSlice(s.propose), Commit:toSlice(s.commit), Reveal:toSlice(s.reveal), Ack:toSlice(s.ack), Done:s.done}
}

func (e *EngineImpl) restore(st sessionState) *session {
    toMap := func(a []string) map[string]struct{} { m:=map[string]struct{}{}; for _,k := range a { m[k]=struct{}{} }; return m }
    return &session{epoch:st.Epoch, propose:toMap(st.Propose), commit:toMap(st.Commit), reveal:toMap(st.Reveal), ack:toMap(st.Ack), done:st.Done}
}
    commit  map[string]struct{}
    reveal  map[string]struct{}
    ack     map[string]struct{}
    done    bool
}

// NewEngine constructs a new minimal DKG engine.
func NewEngine(cfg Config) *EngineImpl {\n    if cfg.T <= 0 { cfg.T = 2 }\n    return &EngineImpl{cfg: cfg, sess: make(map[string]*session)}\n}\n
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
    case MsgComplaint:\n        // Complaint triggers epoch bump and phase reset (retry)\n        s.epoch++\n        s.propose = map[string]struct{}{}\n        s.commit = map[string]struct{}{}\n        s.reveal = map[string]struct{}{}\n        s.ack = map[string]struct{}{}\n        // persist below\n    default:
        // Unknown types ignored
    }
    // persist session snapshot if store provided\n    if e.cfg.Sess != nil { _ = e.cfg.Sess.Save(msg.SessionID, e.snapshot(s)) }\n    e.mu.Unlock()\n    return advanced, nil
}

\n// Resume loads a session state from store (if provided) into memory.\nfunc (e *EngineImpl) Resume(id string) error {\n    if e.cfg.Sess == nil { return ErrSessNotFound }\n    st, err := e.cfg.Sess.Load(id); if err != nil { return err }\n    e.mu.Lock(); e.sess[id] = e.restore(st); e.mu.Unlock()\n    return nil\n}\n