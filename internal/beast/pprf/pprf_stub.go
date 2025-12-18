//go:build !blst

package pprf

import "errors"

var ErrNotSupported = errors.New("pprf requires -tags blst")

type LinearParams struct {
	N      int
	G1Pows [][]byte
	G2Pows [][]byte
}

func SetupLinear(_ int) (LinearParams, error) { return LinearParams{}, ErrNotSupported }
func KeyGen() ([]byte, error)                 { return nil, ErrNotSupported }
func AddKeys(_ ...[]byte) ([]byte, error)     { return nil, ErrNotSupported }
func Eval(_ LinearParams, _ []byte, _ int) ([]byte, error) {
	return nil, ErrNotSupported
}
func Puncture(_ LinearParams, _ []byte, _ int) ([]byte, error) {
	return nil, ErrNotSupported
}
func PuncturedEval(_ LinearParams, _ []byte, _, _ int) ([]byte, error) {
	return nil, ErrNotSupported
}
