package wire

// TopicBeastShare carries per-height BEAST decrypt shares (behind flags).
// The payload is independent of the number of private transactions targeted
// to the same height, enabling batched communication.
const TopicBeastShare = "aequa/beast/share/v1"

// BeastShare is a per-height decryption share. Share is expected to be a
// compressed G2 element (96 bytes) in threshold-IBE style flows.
type BeastShare struct {
	Height uint64 `json:"height"`
	Index  int    `json:"index"`
	Share  []byte `json:"share"`
}
