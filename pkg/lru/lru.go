// Package lru implements a generic, fixed-capacity cache that evicts the
// least recently used entry when it is full. All operations are O(1) and safe
// for concurrent use.
package lru

import (
	"container/list"
	"sync"
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

// Cache is an LRU cache. Create one with New.
type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	cap     int
	ll      *list.List // front = most recently used
	items   map[K]*list.Element
	onEvict func(K, V)
}

// New creates a cache that holds at most capacity entries. It panics if
// capacity < 1.
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity < 1 {
		panic("lru: capacity must be >= 1")
	}
	return &Cache[K, V]{cap: capacity, ll: list.New(), items: make(map[K]*list.Element, capacity)}
}

// OnEvict registers fn to be called whenever an entry is evicted because the
// cache is full. It is not called by Remove. fn runs outside the cache lock,
// so it may use the cache.
func (c *Cache[K, V]) OnEvict(fn func(K, V)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = fn
}

// Get returns the value for key and marks it as most recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

// Peek returns the value for key without changing its recency.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		return el.Value.(*entry[K, V]).value, true
	}
	var zero V
	return zero, false
}

// Put inserts or updates key and marks it as most recently used. It reports
// whether another entry had to be evicted to make room.
func (c *Cache[K, V]) Put(key K, value V) (evicted bool) {
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		el.Value.(*entry[K, V]).value = value
		c.ll.MoveToFront(el)
		c.mu.Unlock()
		return false
	}
	c.items[key] = c.ll.PushFront(&entry[K, V]{key, value})
	var old *entry[K, V]
	if c.ll.Len() > c.cap {
		el := c.ll.Back()
		old = c.ll.Remove(el).(*entry[K, V])
		delete(c.items, old.key)
	}
	fn := c.onEvict
	c.mu.Unlock()

	if old != nil && fn != nil {
		fn(old.key, old.value)
	}
	return old != nil
}

// Remove deletes key and reports whether it was present.
func (c *Cache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if ok {
		c.ll.Remove(el)
		delete(c.items, key)
	}
	return ok
}

// Len returns the number of entries currently stored.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

// Keys returns the keys ordered from most to least recently used.
func (c *Cache[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]K, 0, c.ll.Len())
	for el := c.ll.Front(); el != nil; el = el.Next() {
		keys = append(keys, el.Value.(*entry[K, V]).key)
	}
	return keys
}
