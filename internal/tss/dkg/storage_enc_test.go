package dkg

import (
    "context"
    "encoding/hex"
    "os"
    "path/filepath"
    "testing"
)

func tmpDir(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    return dir
}

func mustWrite(t *testing.T, path string, b []byte) {
    t.Helper()
    if err := os.WriteFile(path, b, 0o600); err != nil {
        t.Fatalf("write: %v", err)
    }
}

func TestKeyStore_EncryptRoundtrip(t *testing.T) {
    dir := tmpDir(t)
    path := filepath.Join(dir, "tss_keyshare.dat")
    key := make([]byte, 32)
    for i := range key { key[i] = byte(i) }

    ks := NewKeyStoreEncrypted(path, append([]byte(nil), key...), true)
    want := KeyShare{Index: 7, PublicKey: []byte{1,2,3}, PrivateKey: []byte{4,5}, Commitments: [][]byte{{9}}}
    if err := ks.SaveKeyShare(context.Background(), want); err != nil {
        t.Fatalf("save: %v", err)
    }
    got, err := ks.LoadKeyShare(context.Background())
    if err != nil { t.Fatalf("load: %v", err) }
    if got.Index != want.Index || len(got.PublicKey) != len(want.PublicKey) {
        t.Fatalf("mismatch: %+v vs %+v", got, want)
    }
}

func TestKeyStore_EncryptedFileWithoutKey_FallbackToErr(t *testing.T) {
    dir := tmpDir(t)
    path := filepath.Join(dir, "tss_keyshare.dat")
    // write encrypted file first
    enc := NewKeyStoreEncrypted(path, bytesOf(0xAB, 32), false)
    if err := enc.SaveKeyShare(context.Background(), KeyShare{Index: 1}); err != nil {
        t.Fatalf("save enc: %v", err)
    }
    // reading with non-encrypted store should fail and return ErrNotFound via fallback
    plain := NewKeyStore(path)
    if _, err := plain.LoadKeyShare(context.Background()); err == nil {
        t.Fatalf("expected error without key")
    }
}

func TestKeyStore_FromEnv_HexKey(t *testing.T) {
    dir := tmpDir(t)
    path := filepath.Join(dir, "tss_keyshare.dat")
    raw := bytesOf(0xCD, 32)
    os.Setenv("AEQUA_TSS_KEYSTORE_ENCRYPT", "1")
    os.Setenv("AEQUA_TSS_KEYSTORE_KEY", hex.EncodeToString(raw))
    os.Setenv("AEQUA_TSS_ZEROIZE", "1")
    t.Cleanup(func(){ os.Unsetenv("AEQUA_TSS_KEYSTORE_ENCRYPT"); os.Unsetenv("AEQUA_TSS_KEYSTORE_KEY"); os.Unsetenv("AEQUA_TSS_ZEROIZE") })

    ks := NewKeyStoreFromEnv(path)
    if err := ks.SaveKeyShare(context.Background(), KeyShare{Index: 9}); err != nil {
        t.Fatalf("save: %v", err)
    }
    if _, err := ks.LoadKeyShare(context.Background()); err != nil {
        t.Fatalf("load: %v", err)
    }
}

func TestKeyStore_BakFallback_Encrypted(t *testing.T) {
    dir := tmpDir(t)
    path := filepath.Join(dir, "tss_keyshare.dat")
    ks := NewKeyStoreEncrypted(path, bytesOf(0xEF, 32), false)
    // write first (no .bak yet)
    if err := ks.SaveKeyShare(context.Background(), KeyShare{Index: 1}); err != nil { t.Fatalf("save1: %v", err) }
    // write second (creates .bak of first)
    if err := ks.SaveKeyShare(context.Background(), KeyShare{Index: 2}); err != nil { t.Fatalf("save2: %v", err) }
    // now corrupt main file to force fallback
    mustWrite(t, path, []byte("bad"))
    // expect to read from .bak (Index 2 or 1 depending on rename timing; ensure no error)
    if _, err := ks.LoadKeyShare(context.Background()); err != nil {
        t.Fatalf("fallback load: %v", err)
    }
}

func TestKeyStore_ZeroizeKeySlice(t *testing.T) {
    key := bytesOf(0x11, 32)
    _ = NewKeyStoreEncrypted("/dev/null", key, true)
    for _, b := range key {
        if b != 0 { t.Fatalf("key not zeroized") }
    }
}

// helpers
func bytesOf(v byte, n int) []byte { b := make([]byte, n); for i := range b { b[i]=v }; return b }

