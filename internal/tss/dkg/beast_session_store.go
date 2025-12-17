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
)

// BeastSessionStore persists an in-progress BEAST DKG session so nodes can resume
// after restart without changing their local polynomial.
type BeastSessionStore struct {
	dir string
	mu  sync.Mutex
}

func NewBeastSessionStore(dir string) *BeastSessionStore {
	return &BeastSessionStore{dir: dir}
}

var ErrBeastSessionNotFound = errors.New("beast session not found")

const (
	magicBeastSess uint32 = 0x42445353 // 'BDSS'
	versionBeast   uint16 = 1
)

type beastSessionState struct {
	Epoch uint64 `json:"epoch"`

	// Local polynomial coefficients (32B big-endian scalars).
	Coeffs [][]byte `json:"coeffs,omitempty"`

	// Our commitments (compressed G1 points, 48B each).
	SelfCommitments [][]byte `json:"self_commitments,omitempty"`

	// Remote commitments per dealer index.
	Commitments map[int][][]byte `json:"commitments,omitempty"`

	// Verified dealer->self shares as scalars (32B big-endian).
	Shares map[int][]byte `json:"shares,omitempty"`

	// When done, cache outputs.
	Done        bool   `json:"done,omitempty"`
	GroupPubKey []byte `json:"group_pubkey,omitempty"`
	ShareScalar []byte `json:"share_scalar,omitempty"`
}

func (s *BeastSessionStore) pathFor(sessionID string) string {
	return filepath.Join(s.dir, "beast_dkg_session_"+sessionID+".dat")
}

func writeBeastSession(path string, st beastSessionState) error {
	body, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	var hdr [4 + 2 + 2 + 4 + 4]byte
	off := 0
	binary.BigEndian.PutUint32(hdr[off:], magicBeastSess)
	off += 4
	binary.BigEndian.PutUint16(hdr[off:], versionBeast)
	off += 2
	binary.BigEndian.PutUint16(hdr[off:], 0)
	off += 2
	binary.BigEndian.PutUint32(hdr[off:], uint32(len(body)))
	off += 4
	binary.BigEndian.PutUint32(hdr[off:], crc32.ChecksumIEEE(body))

	if _, err := f.Write(hdr[:]); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// keep previous as .bak if present
	if _, statErr := os.Stat(path); statErr == nil {
		_ = os.Rename(path, path+".bak")
	}
	return os.Rename(tmp, path)
}

func readBeastSession(path string) (beastSessionState, error) {
	f, err := os.Open(path)
	if err != nil {
		return beastSessionState{}, err
	}
	defer f.Close()
	var hdr [4 + 2 + 2 + 4 + 4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return beastSessionState{}, err
	}
	off := 0
	if binary.BigEndian.Uint32(hdr[off:]) != magicBeastSess {
		return beastSessionState{}, errors.New("bad magic")
	}
	off += 4
	_ = binary.BigEndian.Uint16(hdr[off:])
	off += 2
	off += 2
	l := binary.BigEndian.Uint32(hdr[off:])
	off += 4
	want := binary.BigEndian.Uint32(hdr[off:])
	body := make([]byte, int(l))
	if _, err := io.ReadFull(f, body); err != nil {
		return beastSessionState{}, err
	}
	if crc32.ChecksumIEEE(body) != want {
		return beastSessionState{}, errors.New("crc mismatch")
	}
	var st beastSessionState
	if err := json.Unmarshal(body, &st); err != nil {
		return beastSessionState{}, err
	}
	return st, nil
}

func (s *BeastSessionStore) Save(sessionID string, st beastSessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeBeastSession(s.pathFor(sessionID), st)
}

func (s *BeastSessionStore) Load(sessionID string) (beastSessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pathFor(sessionID)
	if st, err := readBeastSession(p); err == nil {
		return st, nil
	}
	if st, err := readBeastSession(p + ".bak"); err == nil {
		return st, nil
	}
	return beastSessionState{}, ErrBeastSessionNotFound
}
