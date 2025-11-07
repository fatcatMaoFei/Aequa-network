package payload

import "sync"

// Container holds a set of typed mempools keyed by payload Type().
type Container struct{
    mu   sync.RWMutex
    impl map[string]TypedMempool
}

// NewContainer constructs a container with the provided type->pool map.
func NewContainer(pools map[string]TypedMempool) *Container {
    return &Container{impl: pools}
}

// Add routes a payload to its typed pool.
func (c *Container) Add(p Payload) error {
    c.mu.RLock(); pool := c.impl[p.Type()]; c.mu.RUnlock()
    if pool == nil { return nil }
    return pool.Add(p)
}

// GetN asks a specific typed pool for up to n payloads.
func (c *Container) GetN(typ string, n int, size int) []Payload {
    c.mu.RLock(); pool := c.impl[typ]; c.mu.RUnlock()
    if pool == nil { return nil }
    return pool.Get(n, size)
}

// Len reports the total size across all typed pools.
func (c *Container) Len() int {
    c.mu.RLock(); defer c.mu.RUnlock()
    sum := 0
    for _, p := range c.impl { sum += p.Len() }
    return sum
}

