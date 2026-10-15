package ttlcache

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a manually driven Clock. Time only moves when Advance is
// called, so tests never sleep and never depend on the machine's speed.
// It is safe for concurrent use because a cache may read it from many goroutines.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// newFakeClock starts at a fixed, arbitrary instant: tests get the same
// timestamps on every run.
func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var _ Clock = (*fakeClock)(nil)

func newCache(t *testing.T) (*Cache[string, int], *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	return New[string, int](clock), clock
}

func TestGetBeforeAndAfterExpiry(t *testing.T) {
	c, clock := newCache(t)
	c.Set("a", 1, time.Minute)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
	}

	clock.Advance(59 * time.Second) // one second left
	if _, ok := c.Get("a"); !ok {
		t.Error("entry expired one second early")
	}

	clock.Advance(time.Second) // exactly at the TTL boundary
	if _, ok := c.Get("a"); ok {
		t.Error("entry still alive at the exact moment of expiry")
	}
}

func TestSetRestartsLifetime(t *testing.T) {
	c, clock := newCache(t)
	c.Set("a", 1, time.Minute)

	clock.Advance(50 * time.Second)
	c.Set("a", 2, time.Minute) // refresh: 60s counted from now

	clock.Advance(50 * time.Second) // 100s since the first Set, 50s since the second
	if v, ok := c.Get("a"); !ok || v != 2 {
		t.Errorf("Get(a) = %d, %v; want 2, true", v, ok)
	}
}

func TestEntriesExpireIndependently(t *testing.T) {
	c, clock := newCache(t)
	c.Set("short", 1, 10*time.Second)
	c.Set("long", 2, time.Hour)

	clock.Advance(time.Minute)

	if _, ok := c.Get("short"); ok {
		t.Error("short entry should have expired")
	}
	if _, ok := c.Get("long"); !ok {
		t.Error("long entry should still be alive")
	}
}

func TestPurge(t *testing.T) {
	c, clock := newCache(t)
	c.Set("a", 1, time.Second)
	c.Set("b", 2, time.Second)
	c.Set("c", 3, time.Hour)

	if got := c.Purge(); got != 0 {
		t.Errorf("Purge() before expiry = %d, want 0", got)
	}

	clock.Advance(time.Minute)
	if got := c.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3 (expired entries linger until purged)", got)
	}
	if got := c.Purge(); got != 2 {
		t.Errorf("Purge() = %d, want 2", got)
	}
	if got := c.Len(); got != 1 {
		t.Errorf("Len() after purge = %d, want 1", got)
	}
}

// The fake clock is shared by goroutines; run with -race to verify it.
func TestConcurrentAccessWithFakeClock(t *testing.T) {
	c, clock := newCache(t)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Set("k", i, time.Minute)
			clock.Advance(time.Millisecond)
			c.Get("k")
		}()
	}
	wg.Wait()

	if _, ok := c.Get("k"); !ok {
		t.Error("entry should be alive: only 8ms passed on the fake clock")
	}
}

// The real clock satisfies the same interface; this only checks the wiring,
// with a TTL so long that no timing can make it flaky.
func TestSystemClock(t *testing.T) {
	c := New[string, int](SystemClock{})
	c.Set("a", 1, time.Hour)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
}
