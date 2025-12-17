//go:build !blst

package private_v1

import "context"

type thresholdSharePublisher func(ctx context.Context, height uint64, index int, share []byte) error

// SetThresholdSharePublisher is a no-op in builds without the blst tag.
func SetThresholdSharePublisher(_ thresholdSharePublisher) {}

// HandleThresholdShare is a no-op in builds without the blst tag.
func HandleThresholdShare(_ uint64, _ int, _ []byte) {}
