package consensus

// ValueRecord captures block value aggregation for downstream sinks.
type ValueRecord struct {
	Height uint64 `json:"height"`
	Round  uint64 `json:"round"`
	Bids   uint64 `json:"bids"`
	Fees   uint64 `json:"fees"`
	Items  int    `json:"items"`
}

// FeeSink defines a non-blocking hook to export block value.
// Implementations must return quickly; errors should be internalized.
type FeeSink interface {
	Publish(ValueRecord)
}

// noopSink is the default sink: no-op.
type noopSink struct{}

func (noopSink) Publish(ValueRecord) {}
