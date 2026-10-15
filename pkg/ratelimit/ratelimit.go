// Package ratelimit implements a token bucket rate limiter with an injectable
// clock, which makes it fully testable without real waiting.
//
// The bucket starts full with burst tokens and refills at rate tokens per
// second, up to burst. Each operation consumes one or more tokens.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Limiter is a token bucket that is safe for concurrent use.
type Limiter struct {
	mu     sync.Mutex
	rate   float64 // tokens per second
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

// Option customizes a Limiter.
type Option func(*Limiter)

// WithClock replaces the time source (default: time.Now).
func WithClock(now func() time.Time) Option { return func(l *Limiter) { l.now = now } }

// New creates a Limiter that refills rate tokens per second and holds at most
// burst tokens. It panics if rate <= 0 or burst < 1.
func New(rate float64, burst int, opts ...Option) *Limiter {
	if rate <= 0 || burst < 1 {
		panic("ratelimit: rate must be > 0 and burst >= 1")
	}
	l := &Limiter{rate: rate, burst: float64(burst), tokens: float64(burst), now: time.Now}
	for _, opt := range opts {
		opt(l)
	}
	l.last = l.now()
	return l
}

// refill adds the tokens accumulated since the last call. Clock regressions
// are ignored. It requires l.mu to be held.
func (l *Limiter) refill() {
	now := l.now()
	if elapsed := now.Sub(l.last); elapsed > 0 {
		l.tokens = math.Min(l.burst, l.tokens+elapsed.Seconds()*l.rate)
		l.last = now
	}
}

// Allow consumes one token, if there is one, and reports whether the
// operation may proceed.
func (l *Limiter) Allow() bool { return l.AllowN(1) }

// AllowN consumes n tokens at once, or none. Requests larger than the burst
// are never allowed.
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	if float64(n) > l.tokens {
		return false
	}
	l.tokens -= float64(n)
	return true
}

// Reserve consumes n tokens even if they do not exist yet and returns how long
// the caller should wait before acting (0 if the tokens are already there).
// The second result is false if n exceeds the burst, in which case nothing is
// consumed.
func (l *Limiter) Reserve(n int) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if float64(n) > l.burst {
		return 0, false
	}
	l.refill()
	l.tokens -= float64(n)
	if l.tokens >= 0 {
		return 0, true
	}
	return time.Duration(-l.tokens / l.rate * float64(time.Second)), true
}

// Tokens reports how many tokens are available right now.
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}
