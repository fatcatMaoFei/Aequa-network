package payload

import (
	"sync"
	"time"
)

// Container holds a set of typed mempools keyed by payload Type().
type Container struct {
	mu   sync.RWMutex
	impl map[string]TypedMempool
	meta map[string]arrivalMeta
	seq  uint64
}

type arrivalMeta struct {
	seq uint64
	ts  time.Time
}

// NewContainer constructs a container with the provided type->pool map.
func NewContainer(pools map[string]TypedMempool) *Container {
	return &Container{impl: pools, meta: map[string]arrivalMeta{}}
}

// Add routes a payload to its typed pool.
func (c *Container) Add(p Payload) error {
	c.mu.Lock()
	pool := c.impl[p.Type()]
	if pool == nil {
		c.mu.Unlock()
		return nil
	}
	c.seq++
	key := string(p.Hash())
	c.meta[key] = arrivalMeta{seq: c.seq, ts: time.Now()}
	c.mu.Unlock()
	return pool.Add(p)
}

// GetN asks a specific typed pool for up to n payloads.
func (c *Container) GetN(typ string, n int, size int) []Payload {
	c.mu.RLock()
	pool := c.impl[typ]
	c.mu.RUnlock()
	if pool == nil {
		return nil
	}
	return pool.Get(n, size)
}

// GetAll returns all payloads for a specific type (best effort).
func (c *Container) GetAll(typ string) []Payload {
	c.mu.RLock()
	pool := c.impl[typ]
	c.mu.RUnlock()
	if pool == nil {
		return nil
	}
	n := pool.Len()
	if n <= 0 {
		return nil
	}
	return pool.Get(n, 0)
}

// Len reports the total size across all typed pools.
func (c *Container) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sum := 0
	for _, p := range c.impl {
		sum += p.Len()
	}
	return sum
}

// Arrival returns arrival metadata if recorded.
func (c *Container) Arrival(p Payload) (arrivalMeta, bool) {
	key := string(p.Hash())
	c.mu.RLock()
	meta, ok := c.meta[key]
	c.mu.RUnlock()
	return meta, ok
}
