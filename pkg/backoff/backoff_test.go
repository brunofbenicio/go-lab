package backoff

import (
	"math"
	"testing"
	"time"
)

const ms = time.Millisecond

func TestStrategies(t *testing.T) {
	tests := []struct {
		name string
		s    Strategy
		want []time.Duration
	}{
		{"constant", Constant(time.Second), []time.Duration{time.Second, time.Second, time.Second}},
		{"linear", Linear(100*ms, 0), []time.Duration{100 * ms, 200 * ms, 300 * ms}},
		{"linear capped", Linear(100*ms, 250*ms), []time.Duration{100 * ms, 200 * ms, 250 * ms, 250 * ms}},
		{"exponential", Exponential(100*ms, 2, 0), []time.Duration{100 * ms, 200 * ms, 400 * ms, 800 * ms}},
		{"exponential capped", Exponential(time.Second, 2, 5*time.Second),
			[]time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}},
		{"exponential factor 1.5", Exponential(100*ms, 1.5, 0), []time.Duration{100 * ms, 150 * ms, 225 * ms}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for attempt, want := range tt.want {
				if got := tt.s(attempt); got != want {
					t.Errorf("attempt %d: got %v, want %v", attempt, got, want)
				}
			}
		})
	}
}

func TestExponentialEdges(t *testing.T) {
	s := Exponential(time.Second, 10, 0)
	if got := s(500); got != math.MaxInt64 {
		t.Errorf("overflow: got %v, want saturation at MaxInt64", got)
	}
	if got, want := s(-3), time.Second; got != want {
		t.Errorf("negative attempt: got %v, want %v", got, want)
	}
	if got := Exponential(time.Second, math.NaN(), 0)(1); got != 0 {
		t.Errorf("NaN factor: got %v, want 0", got)
	}
}

func TestJitter(t *testing.T) {
	base := Constant(time.Second)
	fixed := func(v float64) Rand { return func() float64 { return v } }
	tests := []struct {
		name string
		s    Strategy
		want time.Duration
	}{
		{"full r=0", FullJitter(base, fixed(0)), 0},
		{"full r=0.5", FullJitter(base, fixed(0.5)), 500 * ms},
		{"equal r=0", EqualJitter(base, fixed(0)), 500 * ms},
		{"equal r=0.5", EqualJitter(base, fixed(0.5)), 750 * ms},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s(0); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJitterBoundsWithDefaultRand(t *testing.T) {
	for name, s := range map[string]Strategy{
		"full":  FullJitter(Constant(time.Second), nil),
		"equal": EqualJitter(Constant(time.Second), nil),
	} {
		for i := 0; i < 200; i++ {
			if got := s(0); got < 0 || got > time.Second {
				t.Fatalf("%s: %v is outside [0, 1s]", name, got)
			}
		}
	}
}
