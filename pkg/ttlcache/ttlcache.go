// Package ttlcache implements a generic in-memory cache whose entries expire
// after a time-to-live. Expiration is lazy (checked on access) and the clock
// is injectable, so tests never need to sleep.
package ttlcache

import (
	"sync"
	"time"
)

type options struct{ now func() time.Time }

// Option customizes a Cache.
type Option func(*options)

// WithClock replaces the time source (default: time.Now).
func WithClock(now func() time.Time) Option { return func(o *options) { o.now = now } }

type item[V any] struct {
	value   V
	expires time.Time // zero value: never expires
}

func (it item[V]) expired(now time.Time) bool {
	return !it.expires.IsZero() && !now.Before(it.expires)
}

// Cache stores values with a TTL. It is safe for concurrent use.
type Cache[K comparable, V any] struct {
	mu    sync.Mutex
	items map[K]item[V]
	ttl   time.Duration
	now   func() time.Time
}

// New creates a cache whose entries live for defaultTTL. A defaultTTL <= 0
// means entries stored with Set never expire.
func New[K comparable, V any](defaultTTL time.Duration, opts ...Option) *Cache[K, V] {
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	return &Cache[K, V]{items: make(map[K]item[V]), ttl: defaultTTL, now: o.now}
}

// Set stores value under key using the default TTL.
func (c *Cache[K, V]) Set(key K, value V) { c.SetWithTTL(key, value, c.ttl) }

// SetWithTTL stores value under key for ttl. A ttl <= 0 means no expiration.
// An entry expires exactly ttl after it was stored.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	it := item[V]{value: value}
	if ttl > 0 {
		it.expires = c.now().Add(ttl)
	}
	c.items[key] = it
}

// Get returns the value for key if it exists and has not expired. Expired
// entries are removed on access.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var zero V
	it, ok := c.items[key]
	if !ok {
		return zero, false
	}
	if it.expired(c.now()) {
		delete(c.items, key)
		return zero, false
	}
	return it.value, true
}

// Delete removes key, if present.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Len counts the entries that have not expired. It is O(n).
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now, n := c.now(), 0
	for _, it := range c.items {
		if !it.expired(now) {
			n++
		}
	}
	return n
}

// Purge removes every expired entry and returns how many were removed. Call
// it periodically to free memory held by entries that are never read again.
func (c *Cache[K, V]) Purge() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now, removed := c.now(), 0
	for key, it := range c.items {
		if it.expired(now) {
			delete(c.items, key)
			removed++
		}
	}
	return removed
}
