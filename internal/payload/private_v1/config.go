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
	// Threshold config (optional). When set, the node participates in per-height
	// batched decryption by gossiping a single share per height.
	Threshold int    `json:"threshold,omitempty"`
	Index     int    `json:"index,omitempty"`
	Share     []byte `json:"share,omitempty"` // secret share scalar (32-byte big-endian)
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
	// Threshold mode requires node-specific share material.
	if cfg.Mode == "threshold" || cfg.Threshold > 0 || cfg.Index > 0 || len(cfg.Share) > 0 {
		if len(cfg.GroupPubKey) != 48 {
			return Config{}, errors.New("invalid group pubkey length")
		}
		if cfg.Threshold <= 0 || cfg.Index <= 0 || len(cfg.Share) == 0 {
			return Config{}, errors.New("missing threshold config")
		}
		if len(cfg.Share) != 32 {
			return Config{}, errors.New("invalid share length")
		}
	}
	return cfg, nil
}
