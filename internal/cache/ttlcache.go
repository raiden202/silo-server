// Package cache provides a generic in-process TTL cache backed by a
// sync.RWMutex-protected map with automatic background expiry sweeping.
package cache

import (
	"strings"
	"sync"
	"time"
)

// Default sweep interval for the background expiry goroutine.
const defaultSweepInterval = 30 * time.Second

// ---- TTLCache -----------------------------------------------------------

// entry holds a cached value together with its expiration timestamp.
type entry[V comparable] struct {
	value     V
	expiresAt time.Time
}

// TTLCache is a generic, concurrency-safe, in-process cache with per-key
// time-to-live semantics. A background goroutine periodically sweeps out
// expired entries. Call Close to stop the sweeper when the cache is no
// longer needed.
type TTLCache[V comparable] struct {
	mu      sync.RWMutex
	entries map[string]entry[V]
	stop    chan struct{}
	once    sync.Once // ensures Close is idempotent
}

// NewTTLCache creates a new TTLCache and starts the background sweeper
// with the default 30-second interval.
func NewTTLCache[V comparable]() *TTLCache[V] {
	return newTTLCacheWithSweepInterval[V](defaultSweepInterval)
}

// newTTLCacheWithSweepInterval creates a TTLCache with a caller-specified
// sweep interval. This is intentionally unexported; tests use it to speed
// up sweeper assertions.
func newTTLCacheWithSweepInterval[V comparable](interval time.Duration) *TTLCache[V] {
	c := &TTLCache[V]{
		entries: make(map[string]entry[V]),
		stop:    make(chan struct{}),
	}
	go c.sweepLoop(interval)
	return c
}

// Get retrieves the value for key. It returns the value and true on a hit,
// or the zero value of V and false on a miss (including expired entries).
func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	e, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value under key with the given TTL. If the key already exists
// it is overwritten and the expiry is reset.
func (c *TTLCache[V]) Set(key string, value V, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = entry[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Invalidate removes a single key from the cache immediately.
func (c *TTLCache[V]) Invalidate(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// InvalidatePrefix removes every key that starts with prefix.
func (c *TTLCache[V]) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

// Close stops the background sweeper goroutine. It is safe to call Close
// multiple times; only the first call has any effect.
func (c *TTLCache[V]) Close() {
	c.once.Do(func() {
		close(c.stop)
	})
}

// sweepLoop runs in its own goroutine and periodically removes expired
// entries from the map.
func (c *TTLCache[V]) sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.sweep()
		}
	}
}

// sweep deletes all entries whose expiry time has passed.
func (c *TTLCache[V]) sweep() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}
