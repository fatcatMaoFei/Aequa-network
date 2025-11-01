package p2p

import (
    "errors"
    "fmt"
)

// Config defines minimal, validated parameters required by the P2P service.
// This is PR A: it introduces a single source of truth and strict validation
// executed at service start (fail-fast).
type Config struct {
    // Resource caps
    MaxConns int64

    // Gates (placeholders wired in later PRs)
    AllowList []PeerID
    RateLimit int64
    ScoreThreshold int64

    // DKG/cluster lock expected to be present (NoopVerifier tolerates empty)
    DKGRequired bool

    // TSS limits (S1 占位)：会话并发上限；0 表示不限制（关闭限流）。
    MaxTSSSessions int64
}

// DefaultConfig returns safe defaults compatible with current behaviour.
func DefaultConfig() Config {
    return Config{
        MaxConns:       128,
        AllowList:      nil,
        RateLimit:      0,
        ScoreThreshold: 0,
        DKGRequired:    false,
        MaxTSSSessions: 0,
    }
}

// Validate performs strict checks; return error on any invalid field.
func (c Config) Validate(dkgPresent bool) error {
    if c.MaxConns < 0 {
        return errors.New("maxConns must be >= 0")
    }
    if c.RateLimit < 0 {
        return errors.New("rateLimit must be >= 0")
    }
    if c.ScoreThreshold < 0 {
        return errors.New("scoreThreshold must be >= 0")
    }
    if c.MaxTSSSessions < 0 {
        return errors.New("maxTSSSessions must be >= 0")
    }
    // When DKG is required, a verifier must be wired by caller.
    if c.DKGRequired && !dkgPresent {
        return fmt.Errorf("dkg verifier required but missing")
    }
    return nil
}
