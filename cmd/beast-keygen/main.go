//go:build blst

package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	blst "github.com/supranational/blst/bindings/go"
)

type nodeConfig struct {
	Mode        string `json:"mode"`
	GroupPubKey []byte `json:"group_pubkey"`
	Threshold   int    `json:"threshold"`
	Index       int    `json:"index"`
	Share       []byte `json:"share"`
	BatchN      int    `json:"batch_n,omitempty"`
}

type publicConfig struct {
	GroupPubKey []byte `json:"group_pubkey"`
	Threshold   int    `json:"threshold"`
	N           int    `json:"n"`
}

func main() {
	var (
		n   int
		t   int
		out string
	)
	flag.IntVar(&n, "n", 4, "Total participants")
	flag.IntVar(&t, "t", 3, "Threshold (t-of-n)")
	flag.StringVar(&out, "out", "beast-keys", "Output directory")
	flag.Parse()

	if n <= 0 || t <= 0 || t > n {
		fmt.Fprintln(os.Stderr, "invalid n/t")
		os.Exit(2)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	msk, err := randScalar()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	coeffs := make([]*blst.Scalar, t)
	coeffs[0] = msk
	for i := 1; i < t; i++ {
		s, err := randScalar()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		coeffs[i] = s
	}

	var gpk blst.P1Affine
	gpk.From(msk)
	gpkBytes := gpk.Compress()

	pub := publicConfig{GroupPubKey: gpkBytes, Threshold: t, N: n}
	if err := writeJSON(filepath.Join(out, "beast-public.json"), pub); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for i := 1; i <= n; i++ {
		share, err := evalPoly(coeffs, i)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		cfg := nodeConfig{
			Mode:        "threshold",
			GroupPubKey: gpkBytes,
			Threshold:   t,
			Index:       i,
			Share:       share.Serialize(),
			BatchN:      n,
		}
		path := filepath.Join(out, fmt.Sprintf("beast-node-%d.json", i))
		if err := writeJSON(path, cfg); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	fmt.Printf("wrote %d configs to %s\n", n+1, out)
}

func randScalar() (*blst.Scalar, error) {
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ikm); err != nil {
		return nil, err
	}
	s := blst.KeyGen(ikm)
	if s == nil {
		return nil, errors.New("bad randomness")
	}
	return s, nil
}

func scalarFromInt(v int) *blst.Scalar {
	var buf [blst.BLST_SCALAR_BYTES]byte
	binary.BigEndian.PutUint64(buf[len(buf)-8:], uint64(v))
	var s blst.Scalar
	_ = s.FromBEndian(buf[:])
	return &s
}

func evalPoly(coeffs []*blst.Scalar, x int) (*blst.Scalar, error) {
	if len(coeffs) == 0 {
		return nil, errors.New("empty polynomial")
	}
	xs := scalarFromInt(x)
	res := *coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		if _, ok := (&res).MulAssign(xs); !ok {
			return nil, errors.New("mul failed")
		}
		if _, ok := (&res).AddAssign(coeffs[i]); !ok {
			return nil, errors.New("add failed")
		}
	}
	return &res, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
