package dkg

import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "errors"
    "hash/crc32"
    "io"
    "os"
    "path/filepath"
    "sync"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// KeyStore 提供 KeyShare 的本地持久化，采用原子写（tmp+fsync+rename）与 .bak 回退。
// 可选：启用 AES-256-GCM 加密（默认关闭），并在读写后对敏感内存做 best‑effort 清零。
type KeyStore struct {
    mu      sync.Mutex
    path    string // e.g. tss_keyshare.dat
    aead    cipher.AEAD
    encrypt bool
    zeroize bool
}

// NewKeyStore 构造指定路径的 KeyStore（默认不加密）。
func NewKeyStore(path string) *KeyStore { return &KeyStore{path: path} }

// NewKeyStoreEncrypted 使用给定 32 字节密钥构造开启加密的 KeyStore；
// zeroize 表示读写后尽力将明文缓冲区清零。若 key 长度非法，则回退为不加密。
func NewKeyStoreEncrypted(path string, key []byte, zeroize bool) *KeyStore {
    ks := &KeyStore{path: path}
    if len(key) != 32 {
        return ks
    }
    if a, err := newAESGCM(key); err == nil {
        ks.aead = a
        ks.encrypt = true
        ks.zeroize = zeroize
    }
    zero(key)
    return ks
}

// NewKeyStoreFromEnv 通过环境变量构造 KeyStore（可选加密），默认不开启。
// AEQUA_TSS_KEYSTORE_ENCRYPT=1 开启；密钥通过 AEQUA_TSS_KEYSTORE_KEY（hex 编码 64 字符）
// 或 AEQUA_TSS_KEYSTORE_KEY_FILE（读取原始 32 字节）提供；AEQUA_TSS_ZEROIZE=1 开启内存清零。
func NewKeyStoreFromEnv(path string) *KeyStore {
    if os.Getenv("AEQUA_TSS_KEYSTORE_ENCRYPT") == "1" {
        var key []byte
        if hexStr := os.Getenv("AEQUA_TSS_KEYSTORE_KEY"); hexStr != "" {
            if b, err := hex.DecodeString(hexStr); err == nil {
                key = b
            }
        } else if f := os.Getenv("AEQUA_TSS_KEYSTORE_KEY_FILE"); f != "" {
            if b, err := os.ReadFile(f); err == nil {
                key = b
            }
        }
        zeroize := os.Getenv("AEQUA_TSS_ZEROIZE") == "1"
        return NewKeyStoreEncrypted(path, key, zeroize)
    }
    return NewKeyStore(path)
}

// 错误与文件头常量
var (
    ErrNotFound = errors.New("not found")
)

const (
    magicTSS    uint32 = 0x5453534b // 'TSSK'
    version     uint16 = 1
    flagEncrypt uint16 = 1 << 0
)

// 磁盘结构：
// [magic u32][version u16][flags u16][length u32][crc32 u32][payload ...]
// payload = 若未加密则为 JSON 编码的 KeyShare；若加密则为 nonce(12B)||ciphertext

func (s *KeyStore) writeAtomic(path string, ks KeyShare) error {
    dir := filepath.Dir(path)
    tmp := path + ".tmp"

    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
    if err != nil { return err }
    payload, err := json.Marshal(ks)
    if err != nil { _ = f.Close(); return err }

    // 可选加密
    flags := uint16(0)
    body := payload
    if s.encrypt && s.aead != nil {
        nonce := make([]byte, 12)
        if _, err := rand.Read(nonce); err != nil { _ = f.Close(); zero(payload); return err }
        sealed := s.aead.Seal(nil, nonce, payload, nil)
        body = make([]byte, 0, len(nonce)+len(sealed))
        body = append(body, nonce...)
        body = append(body, sealed...)
        flags |= flagEncrypt
        if s.zeroize { zero(payload) }
    }

    length := uint32(len(body))
    crc := crc32.ChecksumIEEE(body)

    var hdr [4 + 2 + 2 + 4 + 4]byte
    off := 0
    binary.BigEndian.PutUint32(hdr[off:], magicTSS); off += 4
    binary.BigEndian.PutUint16(hdr[off:], version); off += 2
    binary.BigEndian.PutUint16(hdr[off:], flags); off += 2
    binary.BigEndian.PutUint32(hdr[off:], length); off += 4
    binary.BigEndian.PutUint32(hdr[off:], crc)

    if _, err = f.Write(hdr[:]); err != nil { _ = f.Close(); return err }
    if _, err = f.Write(body); err != nil { _ = f.Close(); return err }
    if err = f.Sync(); err != nil { _ = f.Close(); return err }
    if err = f.Close(); err != nil { return err }

    if d, err2 := os.Open(dir); err2 == nil { _ = d.Sync(); _ = d.Close() }

    bak := path + ".bak"
    if _, err := os.Stat(path); err == nil {
        _ = os.Rename(path, bak)
    }
    if err = os.Rename(tmp, path); err != nil { return err }
    if d, err2 := os.Open(dir); err2 == nil { _ = d.Sync(); _ = d.Close() }
    return nil
}

func (s *KeyStore) readFile(path string) (KeyShare, error) {
    f, err := os.Open(path)
    if err != nil { return KeyShare{}, err }
    defer f.Close()
    var hdr [4 + 2 + 2 + 4 + 4]byte
    if _, err = io.ReadFull(f, hdr[:]); err != nil { return KeyShare{}, err }
    off := 0
    mg := binary.BigEndian.Uint32(hdr[off:]); off += 4
    if mg != magicTSS { return KeyShare{}, errors.New("bad magic") }
    _ = binary.BigEndian.Uint16(hdr[off:]); off += 2 // version
    flags := binary.BigEndian.Uint16(hdr[off:]); off += 2
    length := binary.BigEndian.Uint32(hdr[off:]); off += 4
    want := binary.BigEndian.Uint32(hdr[off:])
    if length == 0 { return KeyShare{}, errors.New("bad length") }
    body := make([]byte, int(length))
    if _, err = io.ReadFull(f, body); err != nil { return KeyShare{}, err }
    if got := crc32.ChecksumIEEE(body); got != want { return KeyShare{}, errors.New("crc mismatch") }

    var plain []byte
    if (flags & flagEncrypt) != 0 {
        if s.aead == nil { return KeyShare{}, errors.New("encrypted but no key") }
        if len(body) < 12 { return KeyShare{}, errors.New("bad nonce") }
        nonce, ct := body[:12], body[12:]
        p, err := s.aead.Open(nil, nonce, ct, nil)
        if err != nil { return KeyShare{}, err }
        plain = p
    } else {
        plain = body
    }

    var ks KeyShare
    err = json.Unmarshal(plain, &ks)
    if s.zeroize && len(plain) > 0 { zero(plain) }
    if err != nil { return KeyShare{}, err }
    return ks, nil
}

// SaveKeyShare 持久化 KeyShare
func (s *KeyStore) SaveKeyShare(_ context.Context, ks KeyShare) error {
    begin := time.Now()
    s.mu.Lock(); defer s.mu.Unlock()
    if err := s.writeAtomic(s.path, ks); err != nil {
        metrics.Inc("tss_persist_errors_total", nil)
        logger.ErrorJ("tss_storage", map[string]any{"op":"persist", "result":"error", "err": err.Error(), "trace_id": ""})
        return err
    }
    ms := float64(time.Since(begin).Milliseconds())
    metrics.ObserveSummary("tss_persist_ms", nil, ms)
    logger.InfoJ("tss_storage", map[string]any{"op":"persist", "result":"ok", "latency_ms": ms, "trace_id": ""})
    return nil
}

// LoadKeyShare 读取 KeyShare，若主文件损坏则回退到 .bak。
func (s *KeyStore) LoadKeyShare(_ context.Context) (KeyShare, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    if ks, err := s.readFile(s.path); err == nil {
        metrics.Inc("tss_recovery_total", map[string]string{"result":"ok"})
        logger.InfoJ("tss_storage", map[string]any{"op":"recovery", "result":"ok", "trace_id": ""})
        return ks, nil
    }
    if ks, err := s.readFile(s.path + ".bak"); err == nil {
        metrics.Inc("tss_recovery_total", map[string]string{"result":"fallback"})
        logger.InfoJ("tss_storage", map[string]any{"op":"recovery", "result":"fallback", "trace_id": ""})
        return ks, nil
    }
    metrics.Inc("tss_recovery_total", map[string]string{"result":"fail"})
    logger.InfoJ("tss_storage", map[string]any{"op":"recovery", "result":"miss", "trace_id": ""})
    return KeyShare{}, ErrNotFound
}

// Close 为占位（无状态）
func (s *KeyStore) Close() error { return nil }

// newAESGCM 构造 AES‑256‑GCM 实例。
func newAESGCM(key []byte) (cipher.AEAD, error) {
    block, err := aes.NewCipher(key)
    if err != nil { return nil, err }
    return cipher.NewGCM(block)
}

// zero 尝试将切片内容清零（best‑effort）。
func zero(b []byte) {
    for i := range b {
        b[i] = 0
    }
}

