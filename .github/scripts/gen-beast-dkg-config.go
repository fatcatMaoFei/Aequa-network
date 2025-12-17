package main

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zmlAEQ/Aequa-network/internal/tss/dkg"
)

func main() {
	var (
		outDir      string
		sessionID   string
		epoch       uint64
		n           int
		threshold   int
		keyShare    string
		sessionDir  string
	)
	flag.StringVar(&outDir, "out-dir", "", "Output directory for per-node JSON configs")
	flag.StringVar(&sessionID, "session-id", "beast-dkg", "DKG session id")
	flag.Uint64Var(&epoch, "epoch", 1, "DKG epoch (monotonic)")
	flag.IntVar(&n, "n", 4, "Committee size")
	flag.IntVar(&threshold, "threshold", 3, "Threshold k (<= n)")
	flag.StringVar(&keyShare, "keyshare-path", "/data/tss_keyshare.dat", "KeyShare path inside container")
	flag.StringVar(&sessionDir, "session-dir", "/data/beast_dkg", "Session dir inside container")
	flag.Parse()

	if outDir == "" {
		fmt.Fprintln(os.Stderr, "missing --out-dir")
		os.Exit(2)
	}
	if n <= 0 || threshold <= 0 || threshold > n {
		fmt.Fprintln(os.Stderr, "invalid n/threshold")
		os.Exit(2)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}

	type nodeKeys struct {
		sigPub  []byte
		sigPriv []byte
		encPub  []byte
		encPriv []byte
	}
	keys := make(map[int]nodeKeys, n)
	committee := make([]dkg.BeastDKGMember, 0, n)
	for i := 1; i <= n; i++ {
		sigPub, sigPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ed25519:", err)
			os.Exit(1)
		}
		encPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			fmt.Fprintln(os.Stderr, "x25519:", err)
			os.Exit(1)
		}
		encPub := encPriv.PublicKey()
		keys[i] = nodeKeys{
			sigPub:  append([]byte(nil), sigPub...),
			sigPriv: append([]byte(nil), sigPriv...),
			encPub:  append([]byte(nil), encPub.Bytes()...),
			encPriv: append([]byte(nil), encPriv.Bytes()...),
		}
		committee = append(committee, dkg.BeastDKGMember{
			Index:  i,
			SigPub: append([]byte(nil), sigPub...),
			EncPub: append([]byte(nil), encPub.Bytes()...),
		})
	}

	for i := 1; i <= n; i++ {
		cfg := dkg.BeastDKGConfig{
			SessionID:    sessionID,
			Epoch:        epoch,
			N:            n,
			Threshold:    threshold,
			Index:        i,
			KeySharePath: keyShare,
			SessionDir:   sessionDir,
			SigPriv:      keys[i].sigPriv,
			EncPriv:      keys[i].encPriv,
			Committee:    committee,
		}
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "config validate:", err)
			os.Exit(1)
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal:", err)
			os.Exit(1)
		}
		path := filepath.Join(outDir, fmt.Sprintf("node%d.json", i))
		if err := os.WriteFile(path, b, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}
	}
}

