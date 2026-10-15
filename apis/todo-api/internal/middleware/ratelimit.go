package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimitOptions configures the rate limiting middleware.
type RateLimitOptions struct {
	// RatePerSecond is how many requests per second each client may sustain.
	RatePerSecond float64
	// Burst is how many requests a client may make at once before being
	// throttled. It is also the bucket capacity.
	Burst int
	// KeyFunc identifies the client. The default is the IP in RemoteAddr,
	// which is right when the server faces clients directly. Behind a reverse
	// proxy supply a function that reads a header you trust.
	KeyFunc func(*http.Request) string
	// Now is the clock; it defaults to time.Now and exists for tests.
	Now func() time.Time
}

// RateLimit throttles clients with a token bucket per key. A client that runs
// out of tokens gets 429 with a Retry-After header.
func RateLimit(opts RateLimitOptions) Middleware {
	if opts.KeyFunc == nil {
		opts.KeyFunc = clientIP
	}
	l := newLimiter(opts.RatePerSecond, opts.Burst, opts.Now)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := l.allow(opts.KeyFunc(r))
			if !ok {
				seconds := int(math.Ceil(retryAfter.Seconds()))
				w.Header().Set("Retry-After", strconv.Itoa(max(seconds, 1)))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests, slow down"}}` + "\n"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP returns the host part of the connection's remote address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type bucket struct {
	tokens float64
	last   time.Time
}

// limiter is a set of token buckets keyed by client.
type limiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	rate      float64 // tokens added per second
	burst     float64 // bucket capacity
	now       func() time.Time
	lastSweep time.Time
}

func newLimiter(ratePerSecond float64, burst int, now func() time.Time) *limiter {
	if now == nil {
		now = time.Now
	}
	if ratePerSecond <= 0 {
		ratePerSecond = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &limiter{
		buckets:   make(map[string]*bucket),
		rate:      ratePerSecond,
		burst:     float64(burst),
		now:       now,
		lastSweep: now(),
	}
}

// allow takes one token for key. When the bucket is empty it reports how long
// until a token is available.
func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	missing := 1 - b.tokens
	return false, time.Duration(missing / l.rate * float64(time.Second))
}

// sweep drops buckets that have been idle long enough to be full again, since
// a full bucket is indistinguishable from a brand-new one. It runs at most
// once a minute so the map cannot grow without bound.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now

	refill := time.Duration(l.burst / l.rate * float64(time.Second))
	for key, b := range l.buckets {
		if now.Sub(b.last) >= refill {
			delete(l.buckets, key)
		}
	}
}
