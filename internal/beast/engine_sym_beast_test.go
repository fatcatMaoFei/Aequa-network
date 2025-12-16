//go:build beast

package beast

import "testing"

func TestSymmetricEngine_RoundTrip(t *testing.T) {
	key := []byte("group-key-material")
	e := NewSymmetricEngine(key)

	ct, err := e.Encrypt([]byte("hello"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pt, err := e.Decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(pt) != "hello" {
		t.Fatalf("round trip mismatch: %q", string(pt))
	}
}

