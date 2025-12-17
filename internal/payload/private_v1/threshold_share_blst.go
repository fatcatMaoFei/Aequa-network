//go:build blst

package private_v1

import (
	"context"

	"github.com/zmlAEQ/Aequa-network/internal/beast/ibe"
)

func maybeEnsureShare(height uint64) {
	if height == 0 {
		return
	}
	enabled, idx, _, shareScalar, pub := thresholdParams()
	if !enabled || idx <= 0 || len(shareScalar) == 0 {
		return
	}
	if existing := getShare(height, idx); len(existing) > 0 {
		if pub != nil && markShareSent(height) {
			_ = pub(context.Background(), height, idx, existing)
		}
		return
	}
	id := ibe.IdentityForHeight(height)
	share, err := ibe.DeriveShare(shareScalar, id)
	if err != nil || len(share) == 0 {
		return
	}
	recordLocalShare(height, idx, share)
	if pub != nil && markShareSent(height) {
		_ = pub(context.Background(), height, idx, share)
	}
}
