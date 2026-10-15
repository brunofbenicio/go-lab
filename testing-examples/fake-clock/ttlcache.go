// Package ttlcache is a small in-memory cache whose entries expire after a
// time-to-live. Time comes from an injected Clock, so tests can control it.
package ttlcache

import (
	"sync"
	"time"
)

// Clock abstracts "what time is it". Production code uses SystemClock; tests
// pass a fake they can advance by hand.
type Clock interface {
	Now() time.Time
}

// SystemClock is the real clock.
type SystemClock struct{}

// Now returns time.Now().
func (SystemClock) Now() time.Time { return time.Now() }

type entry[V any] struct {
	value   V
	expires time.Time
}

// Cache maps keys to values that disappear after their TTL. It is safe for
// concurrent use.
type Cache[K comparable, V any] struct {
	clock Clock

	mu      sync.Mutex
	entries map[K]entry[V]
}

// New returns an empty cache reading time from clock.
func New[K comparable, V any](clock Clock) *Cache[K, V] {
	return &Cache[K, V]{clock: clock, entries: make(map[K]entry[V])}
}

// Set stores value under key for ttl, replacing any previous value and
// restarting its lifetime.
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry[V]{value: value, expires: c.clock.Now().Add(ttl)}
}

// Get returns the value for key if it exists and has not expired. An entry is
// expired from the exact instant its lifetime ends.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !c.clock.Now().Before(e.expires) {
		delete(c.entries, key)
		var zero V
		return zero, false
	}
	return e.value, true
}

// Purge removes every expired entry and returns how many were dropped.
func (c *Cache[K, V]) Purge() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now, removed := c.clock.Now(), 0
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
			removed++
		}
	}
	return removed
}

// Len returns the number of stored entries, including expired ones that have
// not been purged yet.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
