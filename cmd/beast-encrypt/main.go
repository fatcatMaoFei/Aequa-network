//go:build blst

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/zmlAEQ/Aequa-network/internal/beast/bte"
	"github.com/zmlAEQ/Aequa-network/internal/beast/ibe"
	"github.com/zmlAEQ/Aequa-network/internal/beast/pprf"
)

type publicConfig struct {
	GroupPubKey []byte `json:"group_pubkey"`
	Threshold   int    `json:"threshold,omitempty"`
	N           int    `json:"n,omitempty"`
}

type innerEnvelope struct {
	Type         string `json:"type"`
	From         string `json:"from"`
	Nonce        uint64 `json:"nonce"`
	Gas          uint64 `json:"gas"`
	Fee          uint64 `json:"fee,omitempty"`
	Bid          uint64 `json:"bid,omitempty"`
	FeeRecipient string `json:"fee_recipient,omitempty"`
}

type outerEnvelope struct {
	Type         string `json:"type"`
	From         string `json:"from"`
	Nonce        uint64 `json:"nonce"`
	Ciphertext   []byte `json:"ciphertext"`
	EphemeralKey []byte `json:"ephemeral_key"`
	TargetHeight uint64 `json:"target_height"`
	BatchIndex   uint64 `json:"batch_index,omitempty"`
	PuncturedKey []byte `json:"punctured_key,omitempty"`
}

func main() {
	var (
		confPath     string
		inPath       string
		targetHeight uint64
		mode         string
	)
	flag.StringVar(&confPath, "conf", "", "Path to beast-public.json (group_pubkey)")
	flag.StringVar(&inPath, "in", "", "Path to inner tx JSON (plaintext_v1 or auction_bid_v1 envelope); default: stdin")
	flag.Uint64Var(&targetHeight, "target-height", 0, "TargetHeight for private_v1")
	flag.StringVar(&mode, "mode", "batched", "Encrypt mode: 'ibe' (threshold IBE) or 'batched' (BTE+PPRF, experimental)")
	flag.Parse()

	if confPath == "" || targetHeight == 0 {
		fmt.Fprintln(os.Stderr, "missing --conf or --target-height")
		os.Exit(2)
	}
	pub, err := loadPublic(confPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	innerBytes, err := readAll(inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	var inner innerEnvelope
	if err := json.Unmarshal(innerBytes, &inner); err != nil {
		fmt.Fprintln(os.Stderr, "invalid inner json")
		os.Exit(2)
	}
	if inner.From == "" {
		fmt.Fprintln(os.Stderr, "inner.from required")
		os.Exit(2)
	}
	if inner.Type == "" {
		inner.Type = "plaintext_v1"
	}
	// Encrypt the canonical JSON encoding of the inner envelope.
	pt, _ := json.Marshal(inner)
	var out outerEnvelope
	switch mode {
	case "ibe":
		id := ibe.IdentityForHeight(targetHeight)
		eph, ct, err := ibe.Encrypt(pub.GroupPubKey, id, pt)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		out = outerEnvelope{
			Type:         "private_v1",
			From:         inner.From,
			Nonce:        inner.Nonce,
			Ciphertext:   ct,
			EphemeralKey: eph,
			TargetHeight: targetHeight,
		}
	case "batched":
		if pub.N <= 0 {
			fmt.Fprintln(os.Stderr, "batched mode requires n>0 in beast-public.json")
			os.Exit(2)
		}
		pp, err := pprf.SetupLinearDeterministic(pub.N, pub.GroupPubKey)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		key, err := pprf.KeyGen()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		// Deterministic index in [1..N] to keep envelope small; can be
		// refined in later protocol iterations.
		idx := 1
		if pub.N > 0 {
			idx = int(inner.Nonce%uint64(pub.N)) + 1
		}
		prf, err := pprf.Eval(pp, key, idx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		beta := bte.RecoverXOR(pt, prf)
		ct, err := bte.EncryptKey(pub.GroupPubKey, key)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		kStar, err := pprf.Puncture(pp, key, idx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		eph := append(append([]byte(nil), ct.C1...), ct.C2...)
		out = outerEnvelope{
			Type:         "private_v1",
			From:         inner.From,
			Nonce:        inner.Nonce,
			Ciphertext:   beta,
			EphemeralKey: eph,
			TargetHeight: targetHeight,
			BatchIndex:   uint64(idx),
			PuncturedKey: kStar,
		}
	default:
		fmt.Fprintln(os.Stderr, "unsupported mode: "+mode)
		os.Exit(2)
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func loadPublic(path string) (publicConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return publicConfig{}, err
	}
	var cfg publicConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return publicConfig{}, err
	}
	if len(cfg.GroupPubKey) == 0 {
		return publicConfig{}, fmt.Errorf("missing group_pubkey")
	}
	if len(cfg.GroupPubKey) != 48 {
		return publicConfig{}, fmt.Errorf("invalid group_pubkey length")
	}
	return cfg, nil
}

func readAll(path string) ([]byte, error) {
	if path == "" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
