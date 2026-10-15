// Package lru implements a generic least-recently-used cache.
//
// A Cache is not safe for concurrent use; wrap it with a mutex if several
// goroutines share it.
package lru

import (
	"container/list"
	"errors"
)

// ErrCapacity is returned by New when the capacity is not positive.
var ErrCapacity = errors.New("lru: capacity must be positive")

type entry[K comparable, V any] struct {
	key   K
	value V
}

// Cache holds up to a fixed number of entries. When it is full, adding a new
// key evicts the entry that was used least recently.
type Cache[K comparable, V any] struct {
	capacity int
	order    *list.List          // front = most recently used
	items    map[K]*list.Element // key -> element of order holding *entry
	onEvict  func(K, V)
}

// New creates a cache with room for capacity entries. onEvict, if not nil, is
// called with every entry removed to make room (not for Delete or Clear).
func New[K comparable, V any](capacity int, onEvict func(key K, value V)) (*Cache[K, V], error) {
	if capacity <= 0 {
		return nil, ErrCapacity
	}
	return &Cache[K, V]{
		capacity: capacity,
		order:    list.New(),
		items:    make(map[K]*list.Element, capacity),
		onEvict:  onEvict,
	}, nil
}

// Get returns the value for key and marks it as most recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*entry[K, V]).value, true
}

// Peek returns the value for key without changing its recency.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	return el.Value.(*entry[K, V]).value, true
}

// Put stores value under key and marks it as most recently used. If the key is
// new and the cache is full, the least recently used entry is evicted first.
func (c *Cache[K, V]) Put(key K, value V) {
	if el, ok := c.items[key]; ok {
		el.Value.(*entry[K, V]).value = value
		c.order.MoveToFront(el)
		return
	}
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		e := c.order.Remove(oldest).(*entry[K, V])
		delete(c.items, e.key)
		if c.onEvict != nil {
			c.onEvict(e.key, e.value)
		}
	}
	c.items[key] = c.order.PushFront(&entry[K, V]{key, value})
}

// Delete removes key and reports whether it was present.
func (c *Cache[K, V]) Delete(key K) bool {
	el, ok := c.items[key]
	if !ok {
		return false
	}
	c.order.Remove(el)
	delete(c.items, key)
	return true
}

// Len returns the number of entries currently stored.
func (c *Cache[K, V]) Len() int { return c.order.Len() }

// Keys returns the keys from most to least recently used.
func (c *Cache[K, V]) Keys() []K {
	keys := make([]K, 0, c.order.Len())
	for el := c.order.Front(); el != nil; el = el.Next() {
		keys = append(keys, el.Value.(*entry[K, V]).key)
	}
	return keys
}
