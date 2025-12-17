package dkg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBeastDKGConfig_Validate_OK(t *testing.T) {
	cfg := BeastDKGConfig{
		SessionID:  "sess",
		N:          4,
		Threshold:  3,
		Index:      2,
		SigPriv:    make([]byte, 64),
		EncPriv:    make([]byte, 32),
		Committee:  []BeastDKGMember{
			{Index: 1, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
			{Index: 2, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
			{Index: 3, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
			{Index: 4, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestBeastDKGConfig_Validate_Errors(t *testing.T) {
	base := BeastDKGConfig{
		SessionID: "sess",
		N:         2,
		Threshold: 2,
		Index:     1,
		SigPriv:   make([]byte, 64),
		EncPriv:   make([]byte, 32),
		Committee: []BeastDKGMember{
			{Index: 1, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
			{Index: 2, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
		},
	}

	cases := []struct {
		name string
		cfg  BeastDKGConfig
	}{
		{"missing_session_id", func() BeastDKGConfig { c := base; c.SessionID = ""; return c }()},
		{"invalid_n", func() BeastDKGConfig { c := base; c.N = 0; return c }()},
		{"invalid_threshold", func() BeastDKGConfig { c := base; c.Threshold = 3; return c }()},
		{"invalid_index", func() BeastDKGConfig { c := base; c.Index = 3; return c }()},
		{"bad_sig_priv", func() BeastDKGConfig { c := base; c.SigPriv = make([]byte, 1); return c }()},
		{"bad_enc_priv", func() BeastDKGConfig { c := base; c.EncPriv = make([]byte, 1); return c }()},
		{"committee_size_mismatch", func() BeastDKGConfig { c := base; c.Committee = c.Committee[:1]; return c }()},
		{"dup_committee_index", func() BeastDKGConfig {
			c := base
			c.Committee = []BeastDKGMember{
				{Index: 1, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
				{Index: 1, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
			}
			return c
		}()},
		{"bad_committee_sig_pub", func() BeastDKGConfig {
			c := base
			c.Committee[0].SigPub = make([]byte, 1)
			return c
		}()},
		{"bad_committee_enc_pub", func() BeastDKGConfig {
			c := base
			c.Committee[0].EncPub = make([]byte, 1)
			return c
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestLoadBeastDKGConfig_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "beast_dkg.json")
	want := BeastDKGConfig{
		SessionID: "sess",
		N:         2,
		Threshold: 2,
		Index:     1,
		SigPriv:   make([]byte, 64),
		EncPriv:   make([]byte, 32),
		Committee: []BeastDKGMember{
			{Index: 1, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
			{Index: 2, SigPub: make([]byte, 32), EncPub: make([]byte, 32)},
		},
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadBeastDKGConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.SessionID != "sess" || cfg.N != 2 || cfg.Threshold != 2 || cfg.Index != 1 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
