package payload

// BlockHeader carries minimal coordinates for deterministic building.
type BlockHeader struct {
	Height uint64
	Round  uint64
}

// BlockStats captures aggregate value for a block selection.
type BlockStats struct {
	TotalFees uint64 // plaintext_v1 fee sum (priority fees)
	TotalBids uint64 // auction_bid_v1 bid sum
	Items     int
}

// StandardBlock is a simple container for selected payloads under a header.
// It intentionally avoids importing concrete payload plugin packages.
type StandardBlock struct {
	Header BlockHeader
	Items  []Payload // selection result in deterministic order
	Stats  BlockStats
}
