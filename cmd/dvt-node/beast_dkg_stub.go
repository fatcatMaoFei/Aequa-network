//go:build !blst

package main

import (
	"context"

	"github.com/zmlAEQ/Aequa-network/internal/p2p"
	"github.com/zmlAEQ/Aequa-network/pkg/logger"
)

func maybeStartBeastDKG(_ context.Context, _ p2p.Transport, confPath string) {
	if confPath == "" {
		return
	}
	logger.Warn("beast dkg requested but 'blst' build tag not enabled; skipping")
}

