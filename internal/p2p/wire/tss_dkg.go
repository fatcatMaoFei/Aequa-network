package wire

// TopicTSSDKG is used to gossip BEAST/TSS DKG messages inside the committee.
// It is intentionally scoped under tss to allow reuse by threshold signing later.
const TopicTSSDKG = "aequa/tss/dkg/v1"

// TSSDKG is a minimal wire message for Feldman-style DKG.
// - Commitments are broadcast (Feldman VSS commitments).
// - Shares are sent encrypted to a specific receiver (ToIndex) but still gossiped
//   via pubsub; non-recipients ignore them.
// - Sig authenticates the message under the sender's configured signing key.
type TSSDKG struct {
	SessionID   string   `json:"session_id"`
	Epoch       uint64   `json:"epoch"`
	Type        string   `json:"type"` // "commitments"|"share"|"ack"|"complaint"
	FromIndex   int      `json:"from_index"`
	ToIndex     int      `json:"to_index,omitempty"`
	Commitments [][]byte `json:"commitments,omitempty"` // compressed G1 points (48B each)
	Nonce       []byte   `json:"nonce,omitempty"`       // AES-GCM nonce (12B) for share messages
	Ciphertext  []byte   `json:"ciphertext,omitempty"`  // encrypted scalar share bytes (32B)
	Sig         []byte   `json:"sig,omitempty"`         // ed25519 signature over unsigned message JSON
}

