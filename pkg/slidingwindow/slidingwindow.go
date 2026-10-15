// Package slidingwindow implements a per-key rate limiter with an exact
// sliding window (sliding window log): it stores the instant of every
// accepted request and counts only those still inside the window.
//
// Unlike a token bucket, it never allows more than limit requests in any
// interval of length window, not even across window boundaries.
package slidingwindow

import (
	"sync"
	"time"
)

// Limiter limits requests per key (user, IP, route...). It is safe for
// concurrent use.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string][]time.Time
}

// Option customizes a Limiter.
type Option func(*Limiter)

// WithClock replaces the time source (default: time.Now).
func WithClock(now func() time.Time) Option { return func(l *Limiter) { l.now = now } }

// New creates a limiter that accepts up to limit requests per key within each
// window of length window. It panics if limit < 1 or window <= 0.
func New(limit int, window time.Duration, opts ...Option) *Limiter {
	if limit < 1 || window <= 0 {
		panic("slidingwindow: limit must be >= 1 and window > 0")
	}
	l := &Limiter{limit: limit, window: window, now: time.Now, hits: make(map[string][]time.Time)}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Allow records a request for key if there is still quota in the window.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	hits := l.prune(key, now)
	if len(hits) >= l.limit {
		return false
	}
	l.hits[key] = append(hits, now)
	return true
}

// Remaining reports how many more requests fit for key in the current window.
func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit - len(l.prune(key, l.now()))
}

// RetryAfter reports how long until key may make a new request; it returns 0
// if there is already quota.
func (l *Limiter) RetryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	hits := l.prune(key, now)
	if len(hits) < l.limit {
		return 0
	}
	return hits[0].Add(l.window).Sub(now)
}

// Cleanup removes keys with no active request, releasing memory.
func (l *Limiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key := range l.hits {
		l.prune(key, now)
	}
}

// prune drops the records outside the window and returns the remaining ones.
// A request made at t counts until (but not including) t+window. It requires
// l.mu to be held.
func (l *Limiter) prune(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	hits := l.hits[key]
	i := 0
	for i < len(hits) && !hits[i].After(cutoff) {
		i++
	}
	if i == len(hits) {
		delete(l.hits, key)
		return nil
	}
	hits = hits[i:]
	l.hits[key] = hits
	return hits
}
