package ratelimit

import (
	"sync"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }

func TestAllow(t *testing.T) {
	type step struct {
		advance time.Duration
		n       int
		want    bool
	}
	tests := []struct {
		name  string
		rate  float64
		burst int
		steps []step
	}{
		{"burst then deny", 1, 3, []step{{0, 1, true}, {0, 1, true}, {0, 1, true}, {0, 1, false}}},
		{"refill one per second", 1, 2, []step{
			{0, 2, true}, {0, 1, false}, {time.Second, 1, true}, {500 * time.Millisecond, 1, false}, {500 * time.Millisecond, 1, true},
		}},
		{"refill capped at burst", 10, 2, []step{{0, 2, true}, {time.Hour, 2, true}, {0, 1, false}}},
		{"request above burst", 1, 2, []step{{0, 3, false}, {0, 2, true}}},
		{"all-or-nothing", 1, 3, []step{{0, 2, true}, {0, 2, false}, {0, 1, true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk := newClock()
			l := New(tt.rate, tt.burst, WithClock(clk.Now))
			for i, s := range tt.steps {
				clk.Advance(s.advance)
				if got := l.AllowN(s.n); got != s.want {
					t.Fatalf("step %d: AllowN(%d) = %v, want %v", i, s.n, got, s.want)
				}
			}
		})
	}
}

func TestReserve(t *testing.T) {
	clk := newClock()
	l := New(2, 2, WithClock(clk.Now))

	if wait, ok := l.Reserve(2); !ok || wait != 0 {
		t.Fatalf("Reserve(2) = %v, %v; want 0, true", wait, ok)
	}
	if wait, ok := l.Reserve(1); !ok || wait != 500*time.Millisecond {
		t.Errorf("Reserve(1) = %v, %v; want 500ms, true", wait, ok)
	}
	if wait, _ := l.Reserve(1); wait != time.Second {
		t.Errorf("second reservation = %v, want 1s", wait)
	}
	if _, ok := l.Reserve(3); ok {
		t.Error("Reserve above the burst should fail")
	}
	clk.Advance(time.Second)
	if wait, _ := l.Reserve(1); wait != 500*time.Millisecond {
		t.Errorf("after 1s: wait = %v, want 500ms", wait)
	}
}

func TestClockGoingBackwardsIsIgnored(t *testing.T) {
	clk := newClock()
	l := New(1, 1, WithClock(clk.Now))
	l.Allow()
	clk.Advance(-time.Hour)
	if l.Allow() {
		t.Error("a clock moving backwards must not add tokens")
	}
}

func TestNewPanics(t *testing.T) {
	for _, tc := range []struct {
		rate  float64
		burst int
	}{{0, 1}, {1, 0}, {-1, 5}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%v, %d) should panic", tc.rate, tc.burst)
				}
			}()
			New(tc.rate, tc.burst)
		}()
	}
}

func TestConcurrentAllowNeverExceedsBurst(t *testing.T) {
	clk := newClock() // frozen clock: only the initial burst is available
	l := New(1, 50, WithClock(clk.Now))
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
	)
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Errorf("allowed = %d, want 50", allowed)
	}
}
