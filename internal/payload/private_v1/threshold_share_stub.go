//go:build !blst

package private_v1

// maybeEnsureShare is a no-op in builds without the blst tag.
func maybeEnsureShare(_ uint64) {}
