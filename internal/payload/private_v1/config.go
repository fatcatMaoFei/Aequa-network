package private_v1

import (
	"encoding/json"
	"errors"
	"os"
)

// Config carries committee/group key material for BEAST decrypt (behind blst tag).
type Config struct {
	Mode        string   `json:"mode,omitempty"`
	GroupPubKey []byte   `json:"group_pubkey"`
	Committee   [][]byte `json:"committee,omitempty"`
	// Threshold config (optional). When Mode=="threshold", the node participates
	// in per-height batched decrypt by gossiping a single share per height.
	Threshold int    `json:"threshold,omitempty"`
	Index     int    `json:"index,omitempty"`
	Share     []byte `json:"share,omitempty"` // secret share scalar (32-byte big-endian)
	// BatchN controls the PPRF/batched BEAST domain size when Mode=="batched".
	// It bounds valid BatchIndex values carried in private_v1 envelopes.
	BatchN int `json:"batch_n,omitempty"`
}

// LoadConfig loads a JSON config from path. Empty path returns an error.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("empty config path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if len(cfg.GroupPubKey) == 0 {
		return Config{}, errors.New("missing group pubkey")
	}
	if len(cfg.GroupPubKey) != 48 {
		return Config{}, errors.New("invalid group pubkey length")
	}
	// Infer mode for legacy configs that omitted Mode but provided threshold fields.
	hasThreshFields := cfg.Threshold > 0 || cfg.Index > 0 || len(cfg.Share) > 0
	if cfg.Mode == "" && hasThreshFields {
		cfg.Mode = "threshold"
	}
	switch cfg.Mode {
	case "threshold":
		if cfg.Threshold <= 0 || cfg.Index <= 0 || len(cfg.Share) == 0 {
			return Config{}, errors.New("missing threshold config")
		}
		if len(cfg.Share) != 32 {
			return Config{}, errors.New("invalid share length")
		}
	case "batched":
		// Batched BEAST requires a domain size and a local decrypt share (single-node
		// threshold for now, i.e. threshold=1).
		if cfg.BatchN <= 0 {
			return Config{}, errors.New("invalid batch_n for batched mode")
		}
		if cfg.Index <= 0 || len(cfg.Share) != 32 {
			return Config{}, errors.New("missing batched decrypt share")
		}
	default:
		// Symmetric / dev modes do not require extra fields.
	}
	return cfg, nil
}
