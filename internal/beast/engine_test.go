package beast

import "testing"

// TestDefaultEngineDisabled ensures that, without any tag or override, the
// global engine reports ErrNotEnabled and performs no encryption.
func TestDefaultEngineDisabled(t *testing.T) {
	t.Cleanup(func() { SetEngine(nil) })

	if _, err := Encrypt([]byte("msg")); err != ErrNotEnabled {
		t.Fatalf("expected ErrNotEnabled, got %v", err)
	}
	if _, err := Decrypt([]byte("cipher")); err != ErrNotEnabled {
		t.Fatalf("expected ErrNotEnabled, got %v", err)
	}
}

