//go:build !blst

package private_v1

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_EmptyPath(t *testing.T) {
	if _, err := LoadConfig(""); err == nil {
		t.Fatalf("expected error on empty path")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(path, []byte("{]"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatalf("expected json error")
	}
}

func TestLoadConfig_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	gpk := base64.StdEncoding.EncodeToString(make([]byte, 48))
	body := fmt.Sprintf(`{"group_pubkey":"%s","committee":["BAUG"]}`, gpk)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.GroupPubKey) == 0 {
		t.Fatalf("missing group pubkey")
	}
	if len(cfg.Committee) != 1 {
		t.Fatalf("committee len mismatch")
	}
}

func TestEnableBLSTDecrypt_Stub(t *testing.T) {
	err := EnableBLSTDecrypt(Config{GroupPubKey: []byte{1}})
	if err == nil {
		t.Fatalf("expected error without blst tag")
	}
}
