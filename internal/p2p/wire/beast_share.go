package wire

// TopicBeastShare carries per-height BEAST decrypt shares (behind flags).
// The payload is independent of the number of private transactions targeted
// to the same height, enabling batched communication.
const TopicBeastShare = "aequa/beast/share/v1"

// BeastShare is a per-height decryption share. For historical threshold-IBE
// flows, Share was a compressed G2 element (96 bytes). For batched BEAST
// flows (BTE), Share carries a compressed G1 element (48 bytes) representing
// a partial decrypt share C1^{s_i}. The Message is kept generic at the wire
// level to avoid metric/log label drift.
type BeastShare struct {
	Height uint64 `json:"height"`
	Index  int    `json:"index"`
	Share  []byte `json:"share"`
}
