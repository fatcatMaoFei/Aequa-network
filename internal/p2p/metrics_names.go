package p2p

// Metric family names reserved for P2P reporting (Phase 1 placeholders).
// Do not increment these in Phase 1; metrics emission must remain unchanged
// until the transport is wired behind a feature flag in later phases.
const (
    MetricP2PMessagesTotal      = "p2p_msgs_total"   // {topic,direction,result}
    MetricP2PBytesTotal         = "p2p_bytes_total"  // {topic,direction}
    MetricConsensusBroadcastTot = "consensus_broadcast_total" // {type,result}
)

