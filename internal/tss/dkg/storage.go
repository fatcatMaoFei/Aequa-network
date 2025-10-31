package dkg

import (
    "context"
    "encoding/binary"
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
type KeyStore struct {
    mu   sync.Mutex
    path string // e.g. tss_keyshare.dat
}

// NewKeyStore 构造指定路径的 KeyStore。
func NewKeyStore(path string) *KeyStore { return &KeyStore{path: path} }

// 错误与文件头常量。
var (
    ErrNotFound = errors.New("not found")
)

const (
    magicTSS uint32 = 0x5453534b // 'TSSK'
    version  uint16 = 1
)

// 磁盘结构：
// [magic u32][version u16][reserved u16][length u32][crc32 u32][payload ...]
// payload = JSON 编码的 KeyShare

func writeAtomic(path string, ks KeyShare) error {
    dir := filepath.Dir(path)
    tmp := path + ".tmp"

    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
    if err != nil { return err }
    payload, err := json.Marshal(ks)
    if err != nil { _ = f.Close(); return err }
    length := uint32(len(payload))
    crc := crc32.ChecksumIEEE(payload)

    var hdr [4 + 2 + 2 + 4 + 4]byte
    off := 0
    binary.BigEndian.PutUint32(hdr[off:], magicTSS); off += 4
    binary.BigEndian.PutUint16(hdr[off:], version); off += 2
    binary.BigEndian.PutUint16(hdr[off:], 0); off += 2 // reserved
    binary.BigEndian.PutUint32(hdr[off:], length); off += 4
    binary.BigEndian.PutUint32(hdr[off:], crc)

    if _, err = f.Write(hdr[:]); err != nil { _ = f.Close(); return err }
    if _, err = f.Write(payload); err != nil { _ = f.Close(); return err }
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

func readFile(path string) (KeyShare, error) {
    f, err := os.Open(path)
    if err != nil { return KeyShare{}, err }
    defer f.Close()
    var hdr [4 + 2 + 2 + 4 + 4]byte
    if _, err = io.ReadFull(f, hdr[:]); err != nil { return KeyShare{}, err }
    off := 0
    mg := binary.BigEndian.Uint32(hdr[off:]); off += 4
    if mg != magicTSS { return KeyShare{}, errors.New("bad magic") }
    _ = binary.BigEndian.Uint16(hdr[off:]); off += 2 // version
    off += 2 // reserved
    length := binary.BigEndian.Uint32(hdr[off:]); off += 4
    want := binary.BigEndian.Uint32(hdr[off:])
    if length == 0 { return KeyShare{}, errors.New("bad length") }
    payload := make([]byte, int(length))
    if _, err = io.ReadFull(f, payload); err != nil { return KeyShare{}, err }
    if got := crc32.ChecksumIEEE(payload); got != want { return KeyShare{}, errors.New("crc mismatch") }
    var ks KeyShare
    if err := json.Unmarshal(payload, &ks); err != nil { return KeyShare{}, err }
    return ks, nil
}

// SaveKeyShare 持久化 KeyShare。
func (s *KeyStore) SaveKeyShare(_ context.Context, ks KeyShare) error {
    begin := time.Now()
    s.mu.Lock(); defer s.mu.Unlock()
    if err := writeAtomic(s.path, ks); err != nil {
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
    if ks, err := readFile(s.path); err == nil {
        metrics.Inc("tss_recovery_total", map[string]string{"result":"ok"})
        logger.InfoJ("tss_storage", map[string]any{"op":"recovery", "result":"ok", "trace_id": ""})
        return ks, nil
    }
    if ks, err := readFile(s.path + ".bak"); err == nil {
        metrics.Inc("tss_recovery_total", map[string]string{"result":"fallback"})
        logger.InfoJ("tss_storage", map[string]any{"op":"recovery", "result":"fallback", "trace_id": ""})
        return ks, nil
    }
    metrics.Inc("tss_recovery_total", map[string]string{"result":"fail"})
    logger.InfoJ("tss_storage", map[string]any{"op":"recovery", "result":"miss", "trace_id": ""})
    return KeyShare{}, ErrNotFound
}

// Close 为占位（无状态）。
func (s *KeyStore) Close() error { return nil }

