package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// manualClock is a clock the test moves by hand.
type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func newManualClock() *manualClock {
	return &manualClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func requestFrom(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	return req
}

func TestRateLimitBurstThenThrottle(t *testing.T) {
	clock := newManualClock()
	h := RateLimit(RateLimitOptions{RatePerSecond: 1, Burst: 3, Now: clock.Now})(okHandler())

	for i := 1; i <= 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom("203.0.113.7:5000"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d within the burst", i, rec.Code, http.StatusOK)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom("203.0.113.7:5000"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d once the burst is spent", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestRateLimitRefills(t *testing.T) {
	clock := newManualClock()
	h := RateLimit(RateLimitOptions{RatePerSecond: 2, Burst: 2, Now: clock.Now})(okHandler())
	send := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom("198.51.100.1:1"))
		return rec.Code
	}

	send()
	send()
	if got := send(); got != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", got)
	}

	clock.Advance(500 * time.Millisecond) // 2 tokens/s -> exactly one token back
	if got := send(); got != http.StatusOK {
		t.Errorf("after 500ms status = %d, want 200", got)
	}
	if got := send(); got != http.StatusTooManyRequests {
		t.Errorf("immediately after, status = %d, want 429 again", got)
	}

	clock.Advance(time.Hour) // the bucket is capped at the burst, not hoarded
	for i := 0; i < 2; i++ {
		if got := send(); got != http.StatusOK {
			t.Errorf("after a long pause request %d status = %d, want 200", i+1, got)
		}
	}
	if got := send(); got != http.StatusTooManyRequests {
		t.Errorf("the refill must be capped at the burst, status = %d, want 429", got)
	}
}

func TestRateLimitRetryAfterReflectsWait(t *testing.T) {
	clock := newManualClock()
	h := RateLimit(RateLimitOptions{RatePerSecond: 0.2, Burst: 1, Now: clock.Now})(okHandler())
	h.ServeHTTP(httptest.NewRecorder(), requestFrom("192.0.2.1:1"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom("192.0.2.1:1"))

	// 0.2 tokens/s means one token every 5 seconds.
	if got, _ := strconv.Atoi(rec.Header().Get("Retry-After")); got != 5 {
		t.Errorf("Retry-After = %q, want 5", rec.Header().Get("Retry-After"))
	}
}

func TestRateLimitIsPerClient(t *testing.T) {
	clock := newManualClock()
	h := RateLimit(RateLimitOptions{RatePerSecond: 1, Burst: 1, Now: clock.Now})(okHandler())

	first := httptest.NewRecorder()
	h.ServeHTTP(first, requestFrom("192.0.2.10:1111"))
	sameIPOtherPort := httptest.NewRecorder()
	h.ServeHTTP(sameIPOtherPort, requestFrom("192.0.2.10:2222"))
	otherIP := httptest.NewRecorder()
	h.ServeHTTP(otherIP, requestFrom("192.0.2.11:1111"))

	if first.Code != http.StatusOK {
		t.Errorf("first client status = %d, want 200", first.Code)
	}
	if sameIPOtherPort.Code != http.StatusTooManyRequests {
		t.Errorf("same IP on another port status = %d, want 429 because the key is the IP", sameIPOtherPort.Code)
	}
	if otherIP.Code != http.StatusOK {
		t.Errorf("a different client status = %d, want 200", otherIP.Code)
	}
}

func TestRateLimitCustomKey(t *testing.T) {
	clock := newManualClock()
	h := RateLimit(RateLimitOptions{
		RatePerSecond: 1, Burst: 1, Now: clock.Now,
		KeyFunc: func(r *http.Request) string { return r.Header.Get("X-API-Key") },
	})(okHandler())

	send := func(key string) int {
		req := requestFrom("192.0.2.50:1")
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if send("alpha") != http.StatusOK || send("beta") != http.StatusOK {
		t.Error("different keys must have independent buckets")
	}
	if send("alpha") != http.StatusTooManyRequests {
		t.Error("the same key must share its bucket")
	}
}

func TestLimiterSweepsIdleBuckets(t *testing.T) {
	clock := newManualClock()
	l := newLimiter(1, 2, clock.Now)
	for _, key := range []string{"a", "b", "c"} {
		l.allow(key)
	}

	clock.Advance(30 * time.Second) // before the one-minute sweep interval
	l.allow("d")
	if len(l.buckets) != 4 {
		t.Fatalf("buckets = %d, want 4 before the sweep interval", len(l.buckets))
	}

	clock.Advance(2 * time.Minute) // a, b, c are long idle and refilled
	l.allow("d")
	if len(l.buckets) != 1 {
		t.Errorf("buckets = %d after the sweep, want only the active one", len(l.buckets))
	}
}

func TestRateLimitConcurrent(t *testing.T) {
	clock := newManualClock()
	const burst = 20
	h := RateLimit(RateLimitOptions{RatePerSecond: 1, Burst: burst, Now: clock.Now})(okHandler())

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, requestFrom("192.0.2.99:1"))
			if rec.Code == http.StatusOK {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != burst {
		t.Errorf("allowed %d requests, want exactly the burst of %d", allowed, burst)
	}
}
