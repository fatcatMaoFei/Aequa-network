package qbft

import (
    "bufio"
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "sync"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// WAL implements a minimal append-only write-ahead log for vote intents
// (prepare/commit). Each entry is one JSON line. This is a best-effort guard
// to reconstruct the last intent on restart and prevent double-sign.
type WAL struct{
    mu   sync.Mutex
    path string
}

type walEntry struct{
    Type   Type   `json:"type"`
    Height uint64 `json:"height"`
    Round  uint64 `json:"round"`
    ID     string `json:"id"`
    From   string `json:"from"`
}

func NewWAL(path string) *WAL { return &WAL{path: path} }

// AppendIntent appends a prepare/commit intent as a single JSON line.
func (w *WAL) AppendIntent(msg Message) error {
    if w == nil { return nil }
    if msg.Type != MsgPrepare && msg.Type != MsgCommit { return nil }
    w.mu.Lock(); defer w.mu.Unlock()
    if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil { return err }
    f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
    if err != nil { return err }
    enc := walEntry{Type: msg.Type, Height: msg.Height, Round: msg.Round, ID: msg.ID, From: msg.From}
    b, _ := json.Marshal(enc)
    if _, err = f.Write(append(b, '\n')); err != nil { _=f.Close(); return err }
    if err = f.Sync(); err != nil { _=f.Close(); return err }
    _ = f.Close()
    metrics.Inc("qbft_wal_appends_total", nil)
    logger.InfoJ("qbft_wal", map[string]any{"op":"append", "result":"ok", "type": string(msg.Type), "height": msg.Height, "round": msg.Round})
    return nil
}

// LastIntent returns the last valid entry from the WAL (if any).
func (w *WAL) LastIntent() (Message, error) {
    if w == nil { return Message{}, errors.New("nil wal") }
    f, err := os.Open(w.path)
    if err != nil { return Message{}, err }
    defer f.Close()
    // Scan all lines and keep the last valid one (files are expected to be small)
    var last Message
    s := bufio.NewScanner(f)
    for s.Scan() {
        var e walEntry
        if json.Unmarshal(s.Bytes(), &e) == nil {
            last = Message{Type: e.Type, Height: e.Height, Round: e.Round, ID: e.ID, From: e.From}
        }
    }
    if last.Type == "" { return Message{}, errors.New("no entries") }
    metrics.Inc("qbft_wal_recover_total", map[string]string{"result":"ok"})
    logger.InfoJ("qbft_wal", map[string]any{"op":"recover", "result":"ok", "type": string(last.Type), "height": last.Height, "round": last.Round})
    return last, nil
}

