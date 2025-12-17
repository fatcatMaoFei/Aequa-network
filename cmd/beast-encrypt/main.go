//go:build blst

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/zmlAEQ/Aequa-network/internal/beast/ibe"
)

type publicConfig struct {
	GroupPubKey []byte `json:"group_pubkey"`
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
}

func main() {
	var (
		confPath     string
		inPath       string
		targetHeight uint64
	)
	flag.StringVar(&confPath, "conf", "", "Path to beast-public.json (group_pubkey)")
	flag.StringVar(&inPath, "in", "", "Path to inner tx JSON (plaintext_v1 or auction_bid_v1 envelope); default: stdin")
	flag.Uint64Var(&targetHeight, "target-height", 0, "TargetHeight for private_v1")
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
	id := ibe.IdentityForHeight(targetHeight)
	eph, ct, err := ibe.Encrypt(pub.GroupPubKey, id, pt)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	out := outerEnvelope{
		Type:         "private_v1",
		From:         inner.From,
		Nonce:        inner.Nonce,
		Ciphertext:   ct,
		EphemeralKey: eph,
		TargetHeight: targetHeight,
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
	return cfg, nil
}

func readAll(path string) ([]byte, error) {
	if path == "" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
