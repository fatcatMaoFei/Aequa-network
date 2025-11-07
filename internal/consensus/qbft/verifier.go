package qbft

import (
    "fmt"
    "sync"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

type Verifier interface {
    Verify(msg Message) error
}

// Policy groups BasicVerifier configuration for easier injection and defaults.
type Policy struct {
    MinHeight     uint64
    RoundWindow   uint64
    ReplayWindow  uint64
    TypeMinHeight map[Type]uint64
    TypeRoundMax  map[Type]uint64
    Allowed       []string
}

// DefaultPolicy returns a zero-valued policy that keeps current behavior.
func DefaultPolicy() Policy { return Policy{} }

type AntiReplay struct {
    mu     sync.Mutex
    seen   map[string]struct{}
    hSeen  map[string]uint64
}

func NewAntiReplay() *AntiReplay { return &AntiReplay{seen: make(map[string]struct{}), hSeen: make(map[string]uint64)} }

// Seen returns true if id already seen; otherwise records and returns false.
func (r *AntiReplay) Seen(id string) bool {
    if id == "" { return false }
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.seen[id]; ok { return true }
    r.seen[id] = struct{}{}
    return false
}

// SeenWithin returns true if id was seen within the given height window.
func (r *AntiReplay) SeenWithin(id string, h, window uint64) bool {
    if id == "" || window == 0 { return false }
    r.mu.Lock()
    defer r.mu.Unlock()
    if last, ok := r.hSeen[id]; ok {
        if h >= last && h-last <= window { return true }
    }
    r.hSeen[id] = h
    return false
}

type BasicVerifier struct {
    replay       *AntiReplay
    minHeight    uint64
    roundWindow  uint64
    allowed      map[string]struct{}
    replayWindow uint64
    // type-scoped windows (placeholders; 0 disables)
    typeMinHeight map[Type]uint64
    typeRoundMax  map[Type]uint64
}

func NewBasicVerifier() *BasicVerifier { return &BasicVerifier{replay: NewAntiReplay()} }
// NewBasicVerifierWithPolicy constructs a BasicVerifier configured from policy.
func NewBasicVerifierWithPolicy(p Policy) *BasicVerifier {
    v := NewBasicVerifier()
    if p.MinHeight > 0 { v.minHeight = p.MinHeight }
    if p.RoundWindow > 0 { v.roundWindow = p.RoundWindow }
    if p.ReplayWindow > 0 { v.replayWindow = p.ReplayWindow }
    if len(p.TypeMinHeight) > 0 { v.typeMinHeight = p.TypeMinHeight }
    if len(p.TypeRoundMax) > 0 { v.typeRoundMax = p.TypeRoundMax }
    if len(p.Allowed) > 0 { v.SetAllowed(p.Allowed...) }
    return v
}

func (v *BasicVerifier) SetMinHeight(h uint64)    { v.minHeight = h }
func (v *BasicVerifier) SetRoundWindow(w uint64)  { v.roundWindow = w }
func (v *BasicVerifier) SetAllowed(ids ...string) {
    if v.allowed == nil { v.allowed = map[string]struct{}{} }
    for _, id := range ids { v.allowed[id] = struct{}{} }
}
func (v *BasicVerifier) SetReplayWindow(w uint64) { v.replayWindow = w }

// SetTypeMinHeight sets a per-type minimum acceptable height (0 disables for that type).
func (v *BasicVerifier) SetTypeMinHeight(t Type, h uint64) {
    if v.typeMinHeight == nil { v.typeMinHeight = map[Type]uint64{} }
    v.typeMinHeight[t] = h
}

// SetTypeRoundMax sets a per-type round upper bound (0 disables for that type).
func (v *BasicVerifier) SetTypeRoundMax(t Type, max uint64) {
    if v.typeRoundMax == nil { v.typeRoundMax = map[Type]uint64{} }
    v.typeRoundMax[t] = max
}

func validType(t Type) bool {
    switch t { case MsgPreprepare, MsgPrepare, MsgCommit, MsgViewChange, MsgNewView: return true }
    return false
}

func (v *BasicVerifier) Verify(msg Message) error {
    labels := map[string]string{"type": string(msg.Type)}
    // structural checks
    if msg.ID == "" || msg.From == "" || !validType(msg.Type) {
        metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"error"})
        logger.ErrorJ("qbft_verify", map[string]any{"result":"error", "reason":"invalid", "type": string(msg.Type), "trace_id": msg.TraceID})
        return fmt.Errorf("invalid message")
    }
    // optional sender whitelist
    if len(v.allowed) > 0 {
        if _, ok := v.allowed[msg.From]; !ok {
            metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"unauthorized"})
            logger.ErrorJ("qbft_verify", map[string]any{"result":"unauthorized", "from": msg.From, "type": string(msg.Type), "trace_id": msg.TraceID})
            return fmt.Errorf("unauthorized")
        }
    }
    // signature shape placeholder (no crypto)
    if l := len(msg.Sig); l > 0 && l < 32 {
        metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"sig_invalid"})
        logger.ErrorJ("qbft_verify", map[string]any{"result":"sig_invalid", "type": string(msg.Type), "trace_id": msg.TraceID})
        return fmt.Errorf("sig invalid")
    }
    // context semantic: preprepare must have round == 0 (placeholder constraint)
    if msg.Type == MsgPreprepare {
        if msg.Round != 0 {
            metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"error"})
            logger.ErrorJ("qbft_verify", map[string]any{"result":"error", "reason":"round_semantic", "type": string(msg.Type), "round": msg.Round, "trace_id": msg.TraceID})
            return fmt.Errorf("invalid round for preprepare")
        }
    }
    // height window (old height)
    if v.minHeight > 0 && msg.Height < v.minHeight {
        metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"old"})
        logger.ErrorJ("qbft_verify", map[string]any{"result":"old", "height": msg.Height, "min": v.minHeight, "type": string(msg.Type), "trace_id": msg.TraceID})
        return fmt.Errorf("old height")
    }
    // round window (upper bound)
    if v.roundWindow > 0 && msg.Round > v.roundWindow {
        metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"round_oob"})
        logger.ErrorJ("qbft_verify", map[string]any{"result":"round_oob", "round": msg.Round, "max": v.roundWindow, "type": string(msg.Type), "trace_id": msg.TraceID})
        return fmt.Errorf("round out of bound")
    }
    // anti-replay: prefer height-windowed replay if configured; otherwise id-level replay
    if v.replay != nil {
        if v.replayWindow > 0 {
            if v.replay.SeenWithin(msg.ID, msg.Height, v.replayWindow) {
                metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"replay"})
                logger.ErrorJ("qbft_verify", map[string]any{"result":"replay", "id": msg.ID, "type": string(msg.Type), "window": v.replayWindow, "trace_id": msg.TraceID})
                return fmt.Errorf("replay")
            }
        } else {
            if v.replay.Seen(msg.ID) {
                metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"replay"})
                logger.ErrorJ("qbft_verify", map[string]any{"result":"replay", "id": msg.ID, "type": string(msg.Type), "trace_id": msg.TraceID})
                return fmt.Errorf("replay")
            }
        }
    }

    // context semantics (placeholder, non-breaking):
    // - preprepare must have round == 0 (added earlier)
    if msg.Type == MsgPreprepare {
        if msg.Round != 0 {
            metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"error"})
            logger.ErrorJ("qbft_verify", map[string]any{"result":"error", "reason":"round_semantic", "type": string(msg.Type), "round": msg.Round, "trace_id": msg.TraceID})
            return fmt.Errorf("invalid round for preprepare")
        }
    }
    // - prepare/commit must have round >= 1
    if msg.Type == MsgPrepare || msg.Type == MsgCommit {
        if msg.Round < 1 {
            metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"error"})
            logger.ErrorJ("qbft_verify", map[string]any{"result":"error", "reason":"round_semantic", "type": string(msg.Type), "round": msg.Round, "trace_id": msg.TraceID})
            return fmt.Errorf("invalid round for %s", msg.Type)
        }
    }

    // type-scoped windows (preserve metric label space; use reason in logs)
    if v.typeMinHeight != nil {
        if min, ok := v.typeMinHeight[msg.Type]; ok && min > 0 && msg.Height < min {
            metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"old"})
            logger.ErrorJ("qbft_verify", map[string]any{"result":"old", "reason":"type_height_old", "type": string(msg.Type), "height": msg.Height, "min": min, "trace_id": msg.TraceID})
            return fmt.Errorf("type-scoped old height")
        }
    }
    if v.typeRoundMax != nil {
        if max, ok := v.typeRoundMax[msg.Type]; ok && max > 0 && msg.Round > max {
            metrics.Inc("qbft_msg_verified_total", map[string]string{"result":"round_oob"})
            logger.ErrorJ("qbft_verify", map[string]any{"result":"round_oob", "reason":"type_round_oob", "type": string(msg.Type), "round": msg.Round, "max": max, "trace_id": msg.TraceID})
            return fmt.Errorf("type-scoped round out of bound")
        }
    }
    // id-level anti-replay handled above when window is disabled
    // ok
    metrics.Inc("qbft_msg_verified_total", labels)
    logger.InfoJ("qbft_verify", map[string]any{"result":"ok", "id": msg.ID, "type": string(msg.Type), "trace_id": msg.TraceID})
    return nil
}

