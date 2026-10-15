package ttlcache

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newCache(ttl time.Duration) (*Cache[string, int], *fakeClock) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	return New[string, int](ttl, WithClock(clk.Now)), clk
}

func TestExpiration(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration // value passed to SetWithTTL; <0 means use Set
		advance time.Duration
		wantOK  bool
	}{
		{"default ttl, before expiry", -1, 59 * time.Second, true},
		{"default ttl, exactly at expiry", -1, time.Minute, false},
		{"custom ttl, before expiry", 10 * time.Second, 9 * time.Second, true},
		{"custom ttl, after expiry", 10 * time.Second, 11 * time.Second, false},
		{"zero ttl never expires", 0, 1000 * time.Hour, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, clk := newCache(time.Minute)
			if tt.ttl < 0 {
				c.Set("k", 1)
			} else {
				c.SetWithTTL("k", 1, tt.ttl)
			}
			clk.Advance(tt.advance)
			if v, ok := c.Get("k"); ok != tt.wantOK || (ok && v != 1) {
				t.Errorf("Get = %d, %v; want ok=%v", v, ok, tt.wantOK)
			}
		})
	}
}

func TestSetRefreshesTTL(t *testing.T) {
	c, clk := newCache(time.Minute)
	c.Set("k", 1)
	clk.Advance(40 * time.Second)
	c.Set("k", 2)
	clk.Advance(40 * time.Second)
	if v, ok := c.Get("k"); !ok || v != 2 {
		t.Errorf("Get = %d, %v; want 2, true", v, ok)
	}
}

func TestLenPurgeDelete(t *testing.T) {
	c, clk := newCache(time.Minute)
	c.Set("old", 1)
	clk.Advance(30 * time.Second)
	c.Set("new", 2)
	c.SetWithTTL("forever", 3, 0)
	clk.Advance(31 * time.Second) // "old" expired, "new" still alive

	if got := c.Len(); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
	if got := c.Purge(); got != 1 {
		t.Errorf("Purge = %d, want 1", got)
	}
	if got := c.Purge(); got != 0 {
		t.Errorf("second Purge = %d, want 0", got)
	}
	c.Delete("new")
	if _, ok := c.Get("new"); ok {
		t.Error("Get after Delete should miss")
	}
	if got := c.Len(); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}
}

func TestGetRemovesExpiredEntry(t *testing.T) {
	c, clk := newCache(time.Second)
	c.Set("k", 1)
	clk.Advance(time.Second)
	c.Get("k")
	if len(c.items) != 0 {
		t.Errorf("expired entry should be removed on access, have %d items", len(c.items))
	}
}
