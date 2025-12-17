package dkg

import (
	"encoding/json"
	"errors"
	"os"
)

// BeastDKGConfig describes a BEAST committee DKG session used to derive
// a BLS12-381 master secret share per node (for threshold decrypt/signing).
//
// NOTE: This is behind runtime flags/tags; default runs do not enable it.
type BeastDKGConfig struct {
	SessionID string `json:"session_id"`
	Epoch     uint64 `json:"epoch,omitempty"`

	// Committee parameters.
	N         int `json:"n"`
	Threshold int `json:"threshold"`
	Index     int `json:"index"`

	// Persistence.
	KeySharePath string `json:"keyshare_path,omitempty"` // default: tss_keyshare.dat
	SessionDir   string `json:"session_dir,omitempty"`   // optional; enables resume/retry

	// Local keys (node-specific).
	SigPriv []byte `json:"sig_priv,omitempty"` // ed25519 private key (64B)
	EncPriv []byte `json:"enc_priv,omitempty"` // X25519 private key (32B)

	// Committee public keys (all nodes).
	Committee []BeastDKGMember `json:"committee"`
}

type BeastDKGMember struct {
	Index  int    `json:"index"`
	SigPub []byte `json:"sig_pub,omitempty"` // ed25519 public key (32B)
	EncPub []byte `json:"enc_pub,omitempty"` // X25519 public key (32B)
}

func LoadBeastDKGConfig(path string) (BeastDKGConfig, error) {
	if path == "" {
		return BeastDKGConfig{}, errors.New("empty config path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return BeastDKGConfig{}, err
	}
	var cfg BeastDKGConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return BeastDKGConfig{}, err
	}
	if err := cfg.Validate(); err != nil {
		return BeastDKGConfig{}, err
	}
	return cfg, nil
}

func (c BeastDKGConfig) Validate() error {
	if c.SessionID == "" {
		return errors.New("missing session_id")
	}
	if c.N <= 0 {
		return errors.New("invalid n")
	}
	if c.Threshold <= 0 || c.Threshold > c.N {
		return errors.New("invalid threshold")
	}
	if c.Index <= 0 || c.Index > c.N {
		return errors.New("invalid index")
	}
	if len(c.SigPriv) != 64 {
		return errors.New("invalid sig_priv")
	}
	if len(c.EncPriv) != 32 {
		return errors.New("invalid enc_priv")
	}
	if len(c.Committee) != c.N {
		return errors.New("committee size mismatch")
	}
	seen := map[int]struct{}{}
	for _, m := range c.Committee {
		if m.Index <= 0 || m.Index > c.N {
			return errors.New("invalid committee index")
		}
		if _, ok := seen[m.Index]; ok {
			return errors.New("duplicate committee index")
		}
		if len(m.SigPub) != 32 {
			return errors.New("invalid committee sig_pub")
		}
		if len(m.EncPub) != 32 {
			return errors.New("invalid committee enc_pub")
		}
		seen[m.Index] = struct{}{}
	}
	return nil
}
