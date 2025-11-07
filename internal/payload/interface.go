package payload

// Payload is the generic payload contract across different mempool plugins.
// Implementations must be deterministic and stable across nodes.
type Payload interface {
    // Type returns a stable type identifier, e.g. "plaintext_v1".
    Type() string
    // Hash returns a stable hash used for deduplication.
    Hash() []byte
    // Validate performs type-specific stateless checks.
    Validate() error
    // SortKey returns a type-specific ordering key (e.g., fee/bid).
    SortKey() uint64
}

// TypedMempool exposes per-type operations; implementations must be safe for
// concurrent use and deterministic in Get() ordering given the same inputs.
type TypedMempool interface {
    Add(p Payload) error
    // Get returns up to n payloads within a size budget (in bytes, advisory).
    Get(n int, size int) []Payload
    Len() int
}

