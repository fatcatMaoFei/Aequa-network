//go:build !blst

package main

import "errors"

var errBLSTNotEnabled = errors.New("blst not enabled")

func beastEncrypt(_ []byte, _ uint64, _ []byte) ([]byte, []byte, error) {
	return nil, nil, errBLSTNotEnabled
}

