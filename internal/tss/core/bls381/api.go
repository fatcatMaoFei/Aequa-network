package bls381

// This package defines a small, testable wrapper API for BLS12-381 operations.
// By default (no build tags, no external deps) it provides stubbed functions
// that return ErrNotImplemented to keep CI stable and dimensions unchanged.
// A future build tag (e.g. "blst") may enable a real implementation using a
// whitelisted library, encapsulated here to avoid cross-package dependencies.

import "errors"

// Errors
var (
    ErrNotImplemented = errors.New("not implemented")
    ErrInvalidInput   = errors.New("invalid input")
)

// Types kept opaque to callers; concrete representation lives behind this
// abstraction and may differ between stub and real builds.
type (
    Scalar    []byte // little-endian scalar bytes
    G1Point   []byte // compressed G1 (48 bytes)
    G2Point   []byte // compressed G2 (96 bytes)
    Signature []byte // compressed G2 (96 bytes)
    PubKey    []byte // compressed G1 (48 bytes)
)

// HashToG2 maps msg to a point in G2 under the provided DST.
func HashToG2(msg, dst []byte) (G2Point, error) { return nil, ErrNotImplemented }

// Verify checks a BLS signature against a pubkey and message under DST.
func Verify(pk PubKey, sig Signature, msg, dst []byte) (bool, error) {
    return false, ErrNotImplemented
}

// Aggregate combines multiple signatures into a single signature.
func Aggregate(sigs ...Signature) (Signature, error) { return nil, ErrNotImplemented }

// VerifyAggregate verifies an aggregate signature for messages (same msg model).
func VerifyAggregate(pks []PubKey, sig Signature, msg, dst []byte) (bool, error) {
    return false, ErrNotImplemented
}

