package slidingwindow

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newLimiter(limit int, window time.Duration) (*Limiter, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	return New(limit, window, WithClock(clk.Now)), clk
}

func TestAllowSlidesWithTime(t *testing.T) {
	type step struct {
		advance time.Duration
		key     string
		want    bool
	}
	tests := []struct {
		name  string
		limit int
		steps []step
	}{
		{"deny after limit", 2, []step{{0, "a", true}, {0, "a", true}, {0, "a", false}}},
		{"keys are independent", 1, []step{{0, "a", true}, {0, "b", true}, {0, "a", false}}},
		{"oldest hit expires first", 2, []step{
			{0, "a", true},
			{4 * time.Second, "a", true},
			{5 * time.Second, "a", false},
			{time.Second, "a", true},     // t=10s: the hit from t=0 left the window
			{0, "a", false},              // still 2 hits in (0s,10s]
			{4 * time.Second, "a", true}, // t=14s: the hit from t=4s left the window
		}},
		{"boundary is exclusive", 1, []step{{0, "a", true}, {10*time.Second - 1, "a", false}, {1, "a", true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, clk := newLimiter(tt.limit, 10*time.Second)
			for i, s := range tt.steps {
				clk.Advance(s.advance)
				if got := l.Allow(s.key); got != s.want {
					t.Fatalf("step %d: Allow(%q) = %v, want %v", i, s.key, got, s.want)
				}
			}
		})
	}
}

func TestRemainingAndRetryAfter(t *testing.T) {
	l, clk := newLimiter(3, 10*time.Second)
	if got := l.Remaining("k"); got != 3 {
		t.Fatalf("initial Remaining = %d, want 3", got)
	}
	for i := 0; i < 3; i++ {
		l.Allow("k")
		clk.Advance(time.Second)
	}
	if got := l.Remaining("k"); got != 0 {
		t.Errorf("Remaining = %d, want 0", got)
	}
	// First hit at t=0, now t=3s: frees up in 7s.
	if got, want := l.RetryAfter("k"), 7*time.Second; got != want {
		t.Errorf("RetryAfter = %v, want %v", got, want)
	}
	if got := l.RetryAfter("other"); got != 0 {
		t.Errorf("RetryAfter for a new key = %v, want 0", got)
	}
}

func TestCleanup(t *testing.T) {
	l, clk := newLimiter(1, time.Second)
	l.Allow("a")
	l.Allow("b")
	clk.Advance(time.Second)
	l.Allow("c")
	l.Cleanup()
	if len(l.hits) != 1 {
		t.Errorf("keys after Cleanup = %d, want 1", len(l.hits))
	}
}

func TestNewPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(0, 0) should panic")
		}
	}()
	New(0, 0)
}
