package qbft

import (
    "fmt"
    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// State represents a minimal QBFT state snapshot.
// This is a skeleton for M3: it carries only coordinates and a textual phase.
type State struct {
    Height uint64
    Round  uint64
    Phase  string // e.g., "idle|preprepared|prepared|commit" (placeholder)
    Leader string // placeholder leader id for current round

    // Minimal aggregation placeholders for M3
    proposalID   string
    prepareVotes map[string]struct{} // by From
    commitVotes  map[string]struct{} // by From
    // View-change aggregation per target round
    viewVotes map[uint64]map[string]struct{}
}

// Processor defines the minimal interface for driving state transitions.
type Processor interface {
    Process(msg Message) error
}

// Process triggers a placeholder state transition based on the incoming message.
// It does not enforce any real QBFT rules; it only updates coordinates,
// emits a log, and increments a Prometheus counter for observability.
func (s *State) Process(msg Message) error {
    // Lightweight, non-authoritative update of coordinates for visibility.
    s.Height = msg.Height
    s.Round = msg.Round
    var ok bool
    changed := false // only count/log transition when state actually changes
    switch msg.Type {
    case MsgPreprepare:
        // Placeholder leader validation: if Leader is set, only accept from that id
        if s.Leader != "" && msg.From != s.Leader {
            logger.ErrorJ("qbft_state", map[string]any{
                "op":        "transition",
                "event_type": string(msg.Type),
                "height":    s.Height,
                "round":     s.Round,
                "reason":    "unauthorized_leader",
                "from":      msg.From,
                "expect":    s.Leader,
                "trace_id":  msg.TraceID,
            })
            return fmt.Errorf("unauthorized leader")
        }
        s.Phase = "preprepared"
        s.proposalID = msg.ID
        s.prepareVotes = make(map[string]struct{})
        s.commitVotes = make(map[string]struct{})
        changed = true
    case MsgPrepare:
        // Strict: require preprepare for this proposal first.
        if s.proposalID == "" || (s.Phase != "preprepared" && s.Phase != "prepared") {
            logger.ErrorJ("qbft_state", map[string]any{
                "op":        "transition",
                "event_type": string(msg.Type),
                "height":    s.Height,
                "round":     s.Round,
                "reason":    "not_preprepared",
                "trace_id":  msg.TraceID,
            })
            return fmt.Errorf("prepare before preprepared")
        }
        if msg.ID != s.proposalID {
            logger.ErrorJ("qbft_state", map[string]any{
                "op":        "transition",
                "event_type": string(msg.Type),
                "height":    s.Height,
                "round":     s.Round,
                "reason":    "proposal_mismatch",
                "got":       msg.ID,
                "expect":    s.proposalID,
                "trace_id":  msg.TraceID,
            })
            return fmt.Errorf("proposal mismatch")
        }
        if _, ok = s.prepareVotes[msg.From]; ok {
            // Duplicate prepare is a no-op regardless of current phase.
            // no-op
            goto END
        }
        s.prepareVotes[msg.From] = struct{}{}
        if s.Phase == "preprepared" && len(s.prepareVotes) >= 2 { // minimal threshold
            s.Phase = "prepared"
            changed = true
            break
        }
        // counted as processed but no phase change if still below threshold
        goto END
    case MsgCommit:
        // Commit is valid for the current proposal after prepared.
        // If already in commit phase for the same proposal, treat duplicates as no-op.
        if s.Phase != "prepared" && s.Phase != "commit" {
            logger.ErrorJ("qbft_state", map[string]any{
                "op":        "transition",
                "event_type": string(msg.Type),
                "height":    s.Height,
                "round":     s.Round,
                "reason":    "not_prepared",
                "trace_id":  msg.TraceID,
            })
            return fmt.Errorf("commit before prepared")
        }
        if msg.ID != s.proposalID {
            logger.ErrorJ("qbft_state", map[string]any{
                "op":        "transition",
                "event_type": string(msg.Type),
                "height":    s.Height,
                "round":     s.Round,
                "reason":    "proposal_mismatch",
                "got":       msg.ID,
                "expect":    s.proposalID,
                "trace_id":  msg.TraceID,
            })
            return fmt.Errorf("proposal mismatch")
        }
        if _, ok = s.commitVotes[msg.From]; ok {
            // Duplicate commit (including when phase already is commit) is a no-op.
            // no-op
            goto END
        }
        s.commitVotes[msg.From] = struct{}{}
        // Minimal rule: first distinct commit advances to commit phase.
        if s.Phase != "commit" && len(s.commitVotes) >= 1 {
            s.Phase = "commit"
            changed = true
        }
    case MsgViewChange:
        // Aggregate view-change votes for the target round (usually current+1)
        if s.viewVotes == nil { s.viewVotes = make(map[uint64]map[string]struct{}) }
        bucket := s.viewVotes[msg.Round]
        if bucket == nil { bucket = map[string]struct{}{}; s.viewVotes[msg.Round] = bucket }
        if _, ok = bucket[msg.From]; ok {
            goto END
        }
        bucket[msg.From] = struct{}{}
        // Minimal threshold 2 to advance the round
        if len(bucket) >= 2 && msg.Round > s.Round {
            s.Round = msg.Round
            // reset phase and votes on view change
            s.Phase = ""
            s.proposalID = ""
            s.prepareVotes = nil
            s.commitVotes = nil
            changed = true
            metrics.Inc("qbft_view_changes_total", nil)
        }
    case MsgNewView:
        // Placeholder: accept new-view as authoritative round update
        if msg.Round > s.Round {
            s.Round = msg.Round
            s.Phase = ""
            s.proposalID = ""
            s.prepareVotes = nil
            s.commitVotes = nil
            changed = true
            metrics.Inc("qbft_view_changes_total", nil)
        }
    default:
        // Keep previous phase for unknown types; still record observability.
    }

END:
    // Observability: log always; count all successfully processed (non-error) messages.
    if changed {
        logger.InfoJ("qbft_state", map[string]any{
            "op":        "transition",
            "event_type": string(msg.Type),
            "height":    s.Height,
            "round":     s.Round,
            "phase":     s.Phase,
            "trace_id":  msg.TraceID,
        })
    } else {
        logger.InfoJ("qbft_state", map[string]any{
            "op":        "transition",
            "event_type": string(msg.Type),
            "height":    s.Height,
            "round":     s.Round,
            "phase":     s.Phase,
            "note":      "noop",
            "trace_id":  msg.TraceID,
        })
    }
    metrics.Inc("qbft_state_transitions_total", map[string]string{"type": string(msg.Type)})
    return nil
}

// OnTimeout records a timeout for the current phase and returns a local view-change
// message targeting the next round. Callers may broadcast the returned message.
func (s *State) OnTimeout() Message {
    phase := s.Phase
    if phase == "" { phase = "idle" }
    metrics.Inc("qbft_timeouts_total", map[string]string{"phase": phase})
    return Message{From: "self", Height: s.Height, Round: s.Round + 1, Type: MsgViewChange, ID: "vc"}
}
