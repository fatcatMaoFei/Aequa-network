//go:build !beast

package beast

// NewSymmetricEngine returns a disabled engine when the 'beast' build tag
// is not enabled. This keeps configuration paths wired without changing
// default behaviour.
func NewSymmetricEngine(_ []byte) Engine {
	return noopEngine{}
}

