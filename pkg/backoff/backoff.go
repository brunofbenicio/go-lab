// Package backoff provides growing wait strategies between retries, with
// jitter variants that keep many clients from retrying in lockstep.
//
// A Strategy receives the attempt index (starting at 0) and returns how long
// to wait before the next attempt. All functions are pure and safe for
// concurrent use.
package backoff

import (
	"math"
	"math/rand/v2"
	"time"
)

// Strategy computes the wait after the attempt with index attempt (0-based).
type Strategy func(attempt int) time.Duration

// Rand returns a pseudo-random number in [0, 1). It is injectable so that
// tests can be deterministic.
type Rand func() float64

// Constant always returns the same wait d.
func Constant(d time.Duration) Strategy {
	return func(int) time.Duration { return d }
}

// Linear grows in fixed steps: step, 2*step, 3*step... If max > 0, the wait
// never exceeds max.
func Linear(step, max time.Duration) Strategy {
	return func(attempt int) time.Duration {
		return clamp(float64(step)*float64(index(attempt)+1), max)
	}
}

// Exponential starts at base and multiplies the wait by factor on every
// attempt: base, base*factor, base*factor², ... If max > 0, the wait never
// exceeds max. Overflows of int64 saturate instead of wrapping.
func Exponential(base time.Duration, factor float64, max time.Duration) Strategy {
	return func(attempt int) time.Duration {
		return clamp(float64(base)*math.Pow(factor, float64(index(attempt))), max)
	}
}

// FullJitter draws the wait uniformly from [0, s(attempt)). If rnd is nil,
// math/rand/v2 is used.
func FullJitter(s Strategy, rnd Rand) Strategy {
	if rnd == nil {
		rnd = rand.Float64
	}
	return func(attempt int) time.Duration {
		return time.Duration(float64(s(attempt)) * rnd())
	}
}

// EqualJitter keeps half of the wait fixed and randomizes the other half:
// s/2 + uniform in [0, s/2). Unlike FullJitter it guarantees a minimum wait.
// If rnd is nil, math/rand/v2 is used.
func EqualJitter(s Strategy, rnd Rand) Strategy {
	if rnd == nil {
		rnd = rand.Float64
	}
	return func(attempt int) time.Duration {
		half := s(attempt) / 2
		return half + time.Duration(float64(half)*rnd())
	}
}

func index(attempt int) int {
	if attempt < 0 {
		return 0
	}
	return attempt
}

// clamp converts d to a Duration honoring the ceiling max (when > 0) and
// saturating instead of overflowing.
func clamp(d float64, max time.Duration) time.Duration {
	switch {
	case math.IsNaN(d) || d < 0:
		return 0
	case max > 0 && d >= float64(max):
		return max
	case d >= math.MaxInt64:
		return math.MaxInt64
	}
	return time.Duration(d)
}
