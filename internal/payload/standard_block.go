package payload

// BlockHeader carries minimal coordinates for deterministic building.
type BlockHeader struct {
    Height uint64
    Round  uint64
}

// StandardBlock is a simple container for selected payloads under a header.
// It intentionally avoids importing concrete payload plugin packages.
type StandardBlock struct {
    Header BlockHeader
    Items  []Payload // selection result in deterministic order
}


