//go:build blst

package private_v1

import (
	"context"
	"sync"
)

type thresholdSharePublisher func(ctx context.Context, height uint64, index int, share []byte) error

var threshMu sync.Mutex

var (
	threshEnabled bool
	threshIndex   int
	threshK       int
	threshShare   []byte
	threshPub     thresholdSharePublisher

	sharesByHeight  = map[uint64]map[int][]byte{}
	sentHeights     = map[uint64]struct{}{}
	privKeyByHeight = map[uint64][]byte{}
)

func enableThreshold(index, k int, share []byte) {
	threshMu.Lock()
	defer threshMu.Unlock()
	threshEnabled = true
	threshIndex = index
	threshK = k
	threshShare = append([]byte(nil), share...)
}

// SetThresholdSharePublisher wires a best-effort publisher used to gossip local
// decrypt shares to the committee. Passing nil disables publishing.
func SetThresholdSharePublisher(fn thresholdSharePublisher) {
	threshMu.Lock()
	defer threshMu.Unlock()
	threshPub = fn
}

// HandleThresholdShare ingests a remote share. It is safe to call from the P2P
// receive loop.
func HandleThresholdShare(height uint64, index int, share []byte) {
	// Expected size for compressed G2 shares is 96 bytes.
	if height == 0 || index <= 0 || len(share) != 96 {
		return
	}
	threshMu.Lock()
	defer threshMu.Unlock()
	m := sharesByHeight[height]
	if m == nil {
		m = map[int][]byte{}
		sharesByHeight[height] = m
	}
	if _, exists := m[index]; exists {
		return
	}
	m[index] = append([]byte(nil), share...)
}

func thresholdParams() (enabled bool, index, k int, share []byte, pub thresholdSharePublisher) {
	threshMu.Lock()
	defer threshMu.Unlock()
	return threshEnabled, threshIndex, threshK, append([]byte(nil), threshShare...), threshPub
}

func markShareSent(height uint64) bool {
	threshMu.Lock()
	defer threshMu.Unlock()
	if _, ok := sentHeights[height]; ok {
		return false
	}
	sentHeights[height] = struct{}{}
	return true
}

func recordLocalShare(height uint64, index int, share []byte) {
	threshMu.Lock()
	defer threshMu.Unlock()
	m := sharesByHeight[height]
	if m == nil {
		m = map[int][]byte{}
		sharesByHeight[height] = m
	}
	m[index] = append([]byte(nil), share...)
}

func getShare(height uint64, index int) []byte {
	threshMu.Lock()
	defer threshMu.Unlock()
	m := sharesByHeight[height]
	if len(m) == 0 {
		return nil
	}
	if b, ok := m[index]; ok && len(b) > 0 {
		return append([]byte(nil), b...)
	}
	return nil
}

func snapshotShares(height uint64) map[int][]byte {
	threshMu.Lock()
	defer threshMu.Unlock()
	src := sharesByHeight[height]
	if len(src) == 0 {
		return nil
	}
	cp := make(map[int][]byte, len(src))
	for k, v := range src {
		cp[k] = append([]byte(nil), v...)
	}
	return cp
}

func getPrivKey(height uint64) []byte {
	threshMu.Lock()
	defer threshMu.Unlock()
	if b, ok := privKeyByHeight[height]; ok && len(b) > 0 {
		return append([]byte(nil), b...)
	}
	return nil
}

func setPrivKey(height uint64, key []byte) {
	threshMu.Lock()
	defer threshMu.Unlock()
	privKeyByHeight[height] = append([]byte(nil), key...)
}
