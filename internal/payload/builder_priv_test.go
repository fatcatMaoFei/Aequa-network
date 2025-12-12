package payload

import (
	"errors"
	"testing"
)

type fakeDecrypter struct {
	called int
	ret    Payload
	err    error
}

func (f *fakeDecrypter) Decrypt(p Payload) (Payload, error) {
	f.called++
	return f.ret, f.err
}

// basic private_v1 decrypt flow should delegate to the registered decrypter and pass through payloads.
func TestDecryptAndMapPrivate_UsesDecrypter(t *testing.T) {
	defer SetPrivateDecrypter(nil) // reset to noop
	fk := &fakeDecrypter{ret: &testPayload{t: "private_v1", key: 1}}
	SetPrivateDecrypter(fk)
	out := decryptAndMapPrivate([]Payload{&testPayload{t: "private_v1", key: 1}})
	if fk.called != 1 {
		t.Fatalf("expected decrypter to be called once, got %d", fk.called)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 payload out, got %d", len(out))
	}
}

// errors from decrypter should drop the payload.
func TestDecryptAndMapPrivate_ErrorsDropped(t *testing.T) {
	defer SetPrivateDecrypter(nil) // reset to noop
	fk := &fakeDecrypter{ret: nil, err: errTest}
	SetPrivateDecrypter(fk)
	out := decryptAndMapPrivate([]Payload{&testPayload{t: "private_v1", key: 1}})
	if len(out) != 0 {
		t.Fatalf("expected dropped payload on error, got %d", len(out))
	}
}

// errTest is a sentinel error for test paths.
var errTest = errors.New("test")

// testPayload is a minimal Payload impl for BEAST path testing.
type testPayload struct {
	t   string
	key uint64
}

func (d *testPayload) Type() string    { return d.t }
func (d *testPayload) Hash() []byte    { return []byte{byte(d.key)} }
func (d *testPayload) Validate() error { return nil }
func (d *testPayload) SortKey() uint64 { return d.key }
