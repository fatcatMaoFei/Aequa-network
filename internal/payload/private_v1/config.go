package private_v1

import (
	"encoding/json"
	"errors"
	"os"
)

// Config carries committee/group key material for BEAST decrypt (behind blst tag).
type Config struct {
	GroupPubKey []byte   `json:"group_pubkey"`
	Committee   [][]byte `json:"committee,omitempty"`
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
	return cfg, nil
}
