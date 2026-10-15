// Package idempotency remembers which webhook deliveries have already been
// accepted, so a delivery that the sender retries is processed only once.
//
// Each delivery has an ID chosen by the sender. The receiver claims the ID
// before doing any work: the first claim wins, later ones are duplicates until
// the entry expires. If processing fails, the receiver releases the ID so the
// sender's retry is processed instead of being dismissed as a duplicate.
//
// Entries live for a fixed time-to-live, which should be longer than the window
// in which the sender may retry (and than twice the timestamp tolerance, see
// package replay). The store is bounded: when it is full, the oldest entry is
// forgotten first.
package idempotency

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// DefaultMaxEntries is the capacity of a Store unless overridden.
const DefaultMaxEntries = 100_000

// Store records delivery IDs. It is safe for concurrent use. Create instances
// with New.
type Store struct {
	ttl        time.Duration
	now        func() time.Time
	maxEntries int

	mu      sync.Mutex
	entries map[string]time.Time // key -> expiry
	queue   []queued             // keys in insertion order, which is expiry order
}

// queued is a record in the insertion-order queue. It can be stale: the key may
// have been released, or released and claimed again with a later expiry.
type queued struct {
	key       string
	expiresAt time.Time
}

// Option customizes a Store.
type Option func(*Store)

// WithClock replaces the time source. A nil function is ignored.
func WithClock(now func() time.Time) Option {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

// WithMaxEntries sets the capacity of the store. New rejects values below one.
func WithMaxEntries(n int) Option {
	return func(s *Store) { s.maxEntries = n }
}

// New returns a Store whose entries live for ttl. It fails if ttl is not
// positive or an option is out of range.
func New(ttl time.Duration, opts ...Option) (*Store, error) {
	if ttl <= 0 {
		return nil, errors.New("idempotency: ttl must be positive")
	}
	s := &Store{
		ttl:        ttl,
		now:        time.Now,
		maxEntries: DefaultMaxEntries,
		entries:    make(map[string]time.Time),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.maxEntries < 1 {
		return nil, fmt.Errorf("idempotency: max entries must be at least 1, got %d", s.maxEntries)
	}
	return s, nil
}

// Claim registers key and reports whether the caller is the first to do so. It
// returns false when key is already registered and has not expired; a
// duplicate claim does not extend the entry's lifetime.
func (s *Store) Claim(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.purge(now)

	if expiresAt, ok := s.entries[key]; ok && now.Before(expiresAt) {
		return false
	}
	for len(s.entries) >= s.maxEntries && len(s.queue) > 0 {
		s.evictOldest()
	}

	expiresAt := now.Add(s.ttl)
	s.entries[key] = expiresAt
	s.queue = append(s.queue, queued{key: key, expiresAt: expiresAt})
	return true
}

// Release forgets key so that it can be claimed again, typically because
// processing the delivery failed. Releasing an unknown key does nothing.
func (s *Store) Release(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
}

// Len returns how many unexpired keys are currently remembered.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purge(s.now())
	return len(s.entries)
}

// purge drops expired entries from the front of the queue. Because every entry
// lives for the same ttl, the queue is ordered by expiry and the scan stops at
// the first live record.
func (s *Store) purge(now time.Time) {
	for len(s.queue) > 0 && !now.Before(s.queue[0].expiresAt) {
		s.evictOldest()
	}
}

// evictOldest removes the record at the front of the queue and, unless the key
// was released or claimed again since, its entry.
func (s *Store) evictOldest() {
	head := s.queue[0]
	s.queue[0] = queued{} // do not keep the key reachable
	s.queue = s.queue[1:]
	if expiresAt, ok := s.entries[head.key]; ok && expiresAt.Equal(head.expiresAt) {
		delete(s.entries, head.key)
	}
}
