//go:build !p2p

package p2p

import (
    "context"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
)

// BuildTransport returns a NoopTransport when built without the 'p2p' tag.
func BuildTransport(_ NetConfig) (Transport, error) {
    logger.Warn("p2p transport requested but 'p2p' build tag not enabled; using NoopTransport")
    return &NoopTransport{}, nil
}

// StartTransportIfEnabled starts the transport when cfg.Enable is true.
// In non-p2p builds, this is a no-op wrapper around NoopTransport.
func StartTransportIfEnabled(ctx context.Context, cfg NetConfig) (Transport, error) {
    if !cfg.Enable {
        return &NoopTransport{}, nil
    }
    t, err := BuildTransport(cfg)
    if err != nil { return &NoopTransport{}, nil }
    if err := t.Start(ctx); err != nil { return &NoopTransport{}, nil }
    return t, nil
}

