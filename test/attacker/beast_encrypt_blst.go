//go:build blst

package main

import "github.com/zmlAEQ/Aequa-network/internal/beast/ibe"

func beastEncrypt(groupPubKey []byte, targetHeight uint64, plaintext []byte) ([]byte, []byte, error) {
	return ibe.Encrypt(groupPubKey, ibe.IdentityForHeight(targetHeight), plaintext)
}

