package circuitbreaker

import (
	"errors"
	"slices"
	"testing"
	"time"
)

var errFail = errors.New("fail")

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newBreaker(threshold int, cooldown time.Duration, opts ...Option) (*Breaker, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	all := append([]Option{WithThreshold(threshold), WithCooldown(cooldown), WithClock(clk.Now)}, opts...)
	return New(all...), clk
}

func fail() error { return errFail }
func ok() error   { return nil }

func TestOpensAfterConsecutiveFailures(t *testing.T) {
	tests := []struct {
		name      string
		calls     []func() error
		wantState State
	}{
		{"below threshold stays closed", []func() error{fail, fail}, Closed},
		{"threshold opens", []func() error{fail, fail, fail}, Open},
		{"success resets the streak", []func() error{fail, fail, ok, fail, fail}, Closed},
		{"opens after reset streak completes", []func() error{fail, ok, fail, fail, fail}, Open},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := newBreaker(3, time.Minute)
			for _, c := range tt.calls {
				_ = b.Execute(c)
			}
			if got := b.State(); got != tt.wantState {
				t.Errorf("State() = %v, want %v", got, tt.wantState)
			}
		})
	}
}

func TestOpenRejectsWithoutCallingFn(t *testing.T) {
	b, clk := newBreaker(1, time.Minute)
	_ = b.Execute(fail)

	called := false
	err := b.Execute(func() error { called = true; return nil })
	if !errors.Is(err, ErrOpen) || called {
		t.Errorf("err = %v, called = %v; want ErrOpen without calling fn", err, called)
	}
	clk.Advance(59 * time.Second)
	if err := b.Execute(ok); !errors.Is(err, ErrOpen) {
		t.Errorf("still inside cooldown: err = %v, want ErrOpen", err)
	}
}

func TestHalfOpenTrial(t *testing.T) {
	t.Run("successful trial closes the circuit", func(t *testing.T) {
		b, clk := newBreaker(1, time.Minute)
		_ = b.Execute(fail)
		clk.Advance(time.Minute)
		if b.State() != HalfOpen {
			t.Fatalf("State() = %v after cooldown, want half-open", b.State())
		}
		if err := b.Execute(ok); err != nil {
			t.Fatalf("trial failed: %v", err)
		}
		if b.State() != Closed {
			t.Errorf("State() = %v, want closed", b.State())
		}
	})

	t.Run("failed trial reopens with a fresh cooldown", func(t *testing.T) {
		b, clk := newBreaker(1, time.Minute)
		_ = b.Execute(fail)
		clk.Advance(time.Minute)
		_ = b.Execute(fail)
		if b.State() != Open {
			t.Fatalf("State() = %v, want open", b.State())
		}
		clk.Advance(30 * time.Second)
		if err := b.Execute(ok); !errors.Is(err, ErrOpen) {
			t.Errorf("cooldown should have restarted: err = %v", err)
		}
	})

	t.Run("only one trial at a time", func(t *testing.T) {
		b, clk := newBreaker(1, time.Minute)
		_ = b.Execute(fail)
		clk.Advance(time.Minute)
		var nested error
		_ = b.Execute(func() error {
			nested = b.Execute(ok) // a second caller while the trial is running
			return nil
		})
		if !errors.Is(nested, ErrOpen) {
			t.Errorf("concurrent call during trial = %v, want ErrOpen", nested)
		}
	})
}

func TestOnStateChange(t *testing.T) {
	var got [][2]State
	b, clk := newBreaker(1, time.Minute, OnStateChange(func(from, to State) {
		got = append(got, [2]State{from, to})
	}))
	_ = b.Execute(fail)
	clk.Advance(time.Minute)
	_ = b.Execute(ok)

	want := [][2]State{{Closed, Open}, {Open, HalfOpen}, {HalfOpen, Closed}}
	if !slices.Equal(got, want) {
		t.Errorf("transitions = %v, want %v", got, want)
	}
}

func TestPanicCountsAsFailureAndPropagates(t *testing.T) {
	b, _ := newBreaker(1, time.Minute)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic should propagate to the caller")
			}
		}()
		_ = b.Execute(func() error { panic("boom") })
	}()
	if b.State() != Open {
		t.Errorf("State() = %v after a panic, want open", b.State())
	}
}
