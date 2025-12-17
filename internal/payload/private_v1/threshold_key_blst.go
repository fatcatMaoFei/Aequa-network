//go:build blst

package private_v1

import "github.com/zmlAEQ/Aequa-network/internal/beast/ibe"

func thresholdPrivateKey(height uint64) ([]byte, bool) {
	if height == 0 {
		return nil, false
	}
	if pk := getPrivKey(height); len(pk) > 0 {
		return pk, true
	}
	enabled, _, k, _, _ := thresholdParams()
	if !enabled || k <= 0 {
		return nil, false
	}
	m := snapshotShares(height)
	if len(m) < k {
		return nil, false
	}
	shares := make([]ibe.Share, 0, len(m))
	for idx, val := range m {
		shares = append(shares, ibe.Share{Index: idx, Value: val})
	}
	pk, err := ibe.CombineShares(shares, k)
	if err != nil || len(pk) == 0 {
		return nil, false
	}
	setPrivKey(height, pk)
	return pk, true
}
