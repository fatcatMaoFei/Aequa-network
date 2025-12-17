//go:build blst

package main

import (
	"context"
	"time"

	"github.com/zmlAEQ/Aequa-network/internal/p2p"
	"github.com/zmlAEQ/Aequa-network/internal/tss/dkg"
	"github.com/zmlAEQ/Aequa-network/pkg/logger"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func maybeStartBeastDKG(ctx context.Context, t p2p.Transport, confPath string) {
	if confPath == "" {
		return
	}
	tr, ok := t.(p2p.TSSDKGTransport)
	if !ok {
		logger.InfoJ("beast_dkg", map[string]any{"result": "skip", "reason": "transport_no_dkg"})
		metrics.Inc("beast_dkg_total", map[string]string{"result": "skip"})
		return
	}
	cfg, err := dkg.LoadBeastDKGConfig(confPath)
	if err != nil {
		logger.InfoJ("beast_dkg", map[string]any{"result": "error", "err": err.Error()})
		metrics.Inc("beast_dkg_total", map[string]string{"result": "config_error"})
		return
	}
	r, err := dkg.NewBeastDKGRunner(cfg, tr)
	if err != nil {
		logger.InfoJ("beast_dkg", map[string]any{"result": "error", "err": err.Error()})
		metrics.Inc("beast_dkg_total", map[string]string{"result": "init_error"})
		return
	}
	if err := r.Start(ctx); err != nil {
		logger.InfoJ("beast_dkg", map[string]any{"result": "error", "err": err.Error()})
		metrics.Inc("beast_dkg_total", map[string]string{"result": "start_error"})
		return
	}

	go func() {
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if res, ok := r.Result(); ok {
					logger.InfoJ("beast_dkg", map[string]any{"result": "ready", "index": res.Index, "threshold": res.Threshold})
					return
				}
			}
		}
	}()
}

