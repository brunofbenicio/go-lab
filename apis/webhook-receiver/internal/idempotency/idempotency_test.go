package idempotency

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

// clock is a manually advanced time source.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: t0} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newStore(t *testing.T, ttl time.Duration, c *clock, opts ...Option) *Store {
	t.Helper()
	s, err := New(ttl, append([]Option{WithClock(c.Now)}, opts...)...)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return s
}

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration
		opts    []Option
		wantErr bool
	}{
		{"valid", time.Hour, nil, false},
		{"zero ttl", 0, nil, true},
		{"negative ttl", -time.Second, nil, true},
		{"capacity of one", time.Hour, []Option{WithMaxEntries(1)}, false},
		{"zero capacity", time.Hour, []Option{WithMaxEntries(0)}, true},
		{"negative capacity", time.Hour, []Option{WithMaxEntries(-3)}, true},
		{"nil clock is ignored", time.Hour, []Option{WithClock(nil)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.ttl, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && s == nil {
				t.Error("New() returned a nil store without an error")
			}
		})
	}
}

func TestClaim(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c)

	steps := []struct {
		name    string
		advance time.Duration
		key     string
		want    bool
	}{
		{"first claim wins", 0, "evt_1", true},
		{"second claim is a duplicate", 0, "evt_1", false},
		{"another key is independent", 0, "evt_2", true},
		{"keys are case-sensitive", 0, "EVT_1", true},
		{"duplicate just before expiry", time.Hour - time.Nanosecond, "evt_1", false},
		{"claimable again exactly at expiry", time.Nanosecond, "evt_1", true},
		{"the renewed claim is a duplicate again", 0, "evt_1", false},
	}
	for _, st := range steps {
		c.Advance(st.advance)
		if got := s.Claim(st.key); got != st.want {
			t.Errorf("%s: Claim(%q) = %v, want %v", st.name, st.key, got, st.want)
		}
	}
}

func TestDuplicateClaimDoesNotExtendTheLifetime(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c)

	s.Claim("evt_1")
	c.Advance(50 * time.Minute)
	if s.Claim("evt_1") {
		t.Fatal("duplicate accepted")
	}
	c.Advance(10 * time.Minute) // one hour after the original claim
	if !s.Claim("evt_1") {
		t.Error("the duplicate attempt extended the lifetime of the entry")
	}
}

func TestRelease(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c)

	if !s.Claim("evt_1") {
		t.Fatal("first claim failed")
	}
	s.Release("evt_1")
	if !s.Claim("evt_1") {
		t.Error("a released key could not be claimed again")
	}
	if s.Claim("evt_1") {
		t.Error("the new claim was not recorded")
	}

	s.Release("never-claimed") // must not panic or affect anything
	if got := s.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}

func TestReleasedAndReclaimedKeyKeepsItsNewLifetime(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c)

	s.Claim("evt_1") // expires at t0+1h
	s.Release("evt_1")
	c.Advance(30 * time.Minute)
	s.Claim("evt_1") // expires at t0+1h30

	// The stale record of the first claim reaches its expiry here and must not
	// take the second claim down with it.
	c.Advance(31 * time.Minute)
	if s.Claim("evt_1") {
		t.Error("the stale record of a released claim evicted the live entry")
	}
	if got := s.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}

func TestLenDropsExpiredEntries(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c)

	for i := range 5 {
		s.Claim(fmt.Sprintf("evt_%d", i))
		c.Advance(10 * time.Minute)
	}
	// Claimed at 0, 10, 20, 30 and 40 minutes; now it is 50 minutes.
	if got := s.Len(); got != 5 {
		t.Fatalf("Len() = %d, want 5", got)
	}

	c.Advance(15 * time.Minute) // 65 minutes: the first claim (0) has expired
	if got := s.Len(); got != 4 {
		t.Errorf("Len() = %d, want 4", got)
	}
	c.Advance(time.Hour)
	if got := s.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

func TestCapacityEvictsTheOldestEntry(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c, WithMaxEntries(3))

	for _, key := range []string{"a", "b", "c"} {
		if !s.Claim(key) {
			t.Fatalf("Claim(%q) failed", key)
		}
		c.Advance(time.Second)
	}
	if !s.Claim("d") { // evicts "a"
		t.Fatal("Claim(\"d\") failed")
	}
	if got := s.Len(); got != 3 {
		t.Fatalf("Len() = %d, want the capacity of 3", got)
	}

	if s.Claim("b") || s.Claim("c") || s.Claim("d") {
		t.Error("a recent entry was forgotten before the oldest one")
	}
	if !s.Claim("a") { // the evicted key is new again; this evicts "b"
		t.Error("the oldest entry was not evicted")
	}
}

func TestCapacitySkipsReleasedRecords(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c, WithMaxEntries(2))

	s.Claim("a")
	s.Claim("b")
	s.Release("a")
	// "a" is gone, so there is room and "b" must survive.
	if !s.Claim("c") {
		t.Fatal("Claim(\"c\") failed")
	}
	if s.Claim("b") {
		t.Error("\"b\" was evicted although a released entry had freed a slot")
	}
}

func TestClockGoingBackwardsIsHarmless(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Hour, c)

	s.Claim("evt_1")
	c.Advance(-24 * time.Hour)
	if s.Claim("evt_1") {
		t.Error("duplicate accepted after the clock moved backwards")
	}
	if !s.Claim("evt_2") {
		t.Error("a new key was refused after the clock moved backwards")
	}
}

func TestConcurrentClaimsOfTheSameKey(t *testing.T) {
	const workers = 64
	s, err := New(time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if s.Claim("evt_shared") {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := wins.Load(); got != 1 {
		t.Errorf("%d claims won, want exactly 1", got)
	}
}

func TestConcurrentMixedUse(t *testing.T) {
	c := newClock()
	s := newStore(t, time.Minute, c, WithMaxEntries(50))

	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				key := fmt.Sprintf("evt_%d", (w*7+i)%80)
				if s.Claim(key) && i%3 == 0 {
					s.Release(key)
				}
				if i%50 == 0 {
					c.Advance(time.Second)
					s.Len()
				}
			}
		}()
	}
	wg.Wait()

	if got := s.Len(); got > 50 {
		t.Errorf("Len() = %d, above the capacity of 50", got)
	}
}
