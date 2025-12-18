//go:build p2p

package p2p

import (
	"context"

	"github.com/zmlAEQ/Aequa-network/pkg/logger"
)

// StartTransportIfEnabled starts the libp2p transport when cfg.Enable is true.
// It mirrors the stub implementation but uses the real BuildTransport.
func StartTransportIfEnabled(ctx context.Context, cfg NetConfig) (Transport, error) {
	if !cfg.Enable {
		return nil, nil
	}
	t, err := BuildTransport(cfg)
	if err != nil {
		logger.ErrorJ("p2p_transport", map[string]any{"result": "error", "err": err.Error()})
		return nil, err
	}
	if err := t.Start(ctx); err != nil {
		logger.ErrorJ("p2p_transport", map[string]any{"result": "start_error", "err": err.Error()})
		return nil, err
	}
	return t, nil
}

