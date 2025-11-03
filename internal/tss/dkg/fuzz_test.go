package dkg

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

// FuzzKeyStore_SaveLoad_NoPanic round-trips random KeyShare values to ensure
// no panics and atomic write/read path behaves.
func FuzzKeyStore_SaveLoad_NoPanic(f *testing.F) {
    f.Add(uint8(1), uint8(1))
    f.Fuzz(func(t *testing.T, a, b uint8) {
        dir := t.TempDir()
        path := filepath.Join(dir, "tss_keyshare.dat")
        s := NewKeyStore(path)
        ks := KeyShare{Index: int(a), PublicKey: []byte{a}, PrivateKey: []byte{b}, Commitments: [][]byte{{a, b}}}
        _ = s.SaveKeyShare(context.Background(), ks)
        _, _ = s.LoadKeyShare(context.Background())
        // corrupt and attempt fallback
        _ = s.SaveKeyShare(context.Background(), KeyShare{Index: int(b)})
        _ = os.Truncate(path, 8)
        _, _ = s.LoadKeyShare(context.Background())
    })
}

