package dkg

import (
    "encoding/binary"
    "encoding/json"
    "errors"
    "hash/crc32"
    "io"
    "os"
    "path/filepath"
    "sync"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

type SessionStore struct { dir string; mu sync.Mutex }

func NewSessionStore(dir string) *SessionStore { return &SessionStore{dir: dir} }

var (
    ErrSessNotFound = errors.New("session not found")
)

const (
    magicSess uint32 = 0x54535353 // 'TSSS'
    versionSess uint16 = 1
)

type sessionState struct {
    Epoch   uint64   `json:"epoch"`
    Propose []string `json:"propose"`
    Commit  []string `json:"commit"`
    Reveal  []string `json:"reveal"`
    Ack     []string `json:"ack"`
    Done    bool     `json:"done"`
}

func (s *SessionStore) pathFor(id string) string { return filepath.Join(s.dir, "tss_session_"+id+".dat") }

func writeSess(path string, st sessionState) error {
    b, err := json.Marshal(st)
    if err != nil { return err }
    tmp := path+".tmp"
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
    if err != nil { return err }
    var hdr [4+2+2+4+4]byte
    off:=0
    binary.BigEndian.PutUint32(hdr[off:], magicSess); off+=4
    binary.BigEndian.PutUint16(hdr[off:], versionSess); off+=2
    binary.BigEndian.PutUint16(hdr[off:], 0); off+=2
    binary.BigEndian.PutUint32(hdr[off:], uint32(len(b))); off+=4
    binary.BigEndian.PutUint32(hdr[off:], crc32.ChecksumIEEE(b))
    if _, err = f.Write(hdr[:]); err != nil { _=f.Close(); return err }
    if _, err = f.Write(b); err != nil { _=f.Close(); return err }
    if err = f.Sync(); err != nil { _=f.Close(); return err }
    if err = f.Close(); err != nil { return err }
    if err = os.Rename(tmp, path); err != nil { return err }
    return nil
}

func readSess(path string) (sessionState, error) {
    f, err := os.Open(path)
    if err != nil { return sessionState{}, err }
    defer f.Close()
    var hdr [4+2+2+4+4]byte
    if _, err = io.ReadFull(f, hdr[:]); err != nil { return sessionState{}, err }
    off:=0
    if binary.BigEndian.Uint32(hdr[off:]) != magicSess { return sessionState{}, errors.New("bad magic") }
    off+=4
    _ = binary.BigEndian.Uint16(hdr[off:]); off+=2
    off+=2
    l := binary.BigEndian.Uint32(hdr[off:]); off+=4
    want := binary.BigEndian.Uint32(hdr[off:])
    body := make([]byte, int(l))
    if _, err = io.ReadFull(f, body); err != nil { return sessionState{}, err }
    if crc32.ChecksumIEEE(body) != want { return sessionState{}, errors.New("crc mismatch") }
    var st sessionState
    if err := json.Unmarshal(body, &st); err != nil { return sessionState{}, err }
    return st, nil
}

func (s *SessionStore) Save(id string, st sessionState) error {
    s.mu.Lock(); defer s.mu.Unlock()
    if err := writeSess(s.pathFor(id), st); err != nil {
        logger.ErrorJ("tss_session", map[string]any{"op":"persist", "result":"error", "err": err.Error()})
        return err
    }
    logger.InfoJ("tss_session", map[string]any{"op":"persist", "result":"ok"})
    return nil
}

func (s *SessionStore) Load(id string) (sessionState, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    st, err := readSess(s.pathFor(id))
    if err != nil { metrics.Inc("tss_recovery_total", map[string]string{"result":"fail"}); return sessionState{}, ErrSessNotFound }
    metrics.Inc("tss_recovery_total", map[string]string{"result":"ok"})
    return st, nil
}