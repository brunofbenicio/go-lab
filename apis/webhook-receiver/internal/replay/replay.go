// Package replay rejects webhook deliveries whose timestamp is too far from the
// current time, which limits how long a captured request stays useful to an
// attacker.
//
// Timestamps are Unix times in whole seconds, written as plain decimal digits
// (for example "1740830400"). The check is symmetric: a timestamp may be up to
// the tolerance in the past (network delays, retries) or in the future (clock
// skew between sender and receiver).
//
// The timestamp is only meaningful if it is authenticated; package signature
// signs it together with the body.
package replay

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	// DefaultTolerance is the customary window of five minutes.
	DefaultTolerance = 5 * time.Minute

	// maxDigits bounds the length of a timestamp: twelve digits reach the year
	// 33658, far beyond anything legitimate.
	maxDigits = 12
)

var (
	// ErrMissing is returned for an empty timestamp.
	ErrMissing = errors.New("replay: timestamp is missing")
	// ErrInvalid is returned for a timestamp that is not a Unix time in
	// seconds made of decimal digits only.
	ErrInvalid = errors.New("replay: timestamp is not a valid Unix time in seconds")
	// ErrTooOld is returned for a timestamp older than the tolerance.
	ErrTooOld = errors.New("replay: timestamp is too old")
	// ErrTooNew is returned for a timestamp further in the future than the
	// tolerance.
	ErrTooNew = errors.New("replay: timestamp is too far in the future")
)

// Checker validates timestamps against a clock. It is safe for concurrent use
// as long as the clock function is.
type Checker struct {
	tolerance time.Duration
	now       func() time.Time
}

// Option customizes a Checker.
type Option func(*Checker)

// WithClock replaces the time source. A nil function is ignored.
func WithClock(now func() time.Time) Option {
	return func(c *Checker) {
		if now != nil {
			c.now = now
		}
	}
}

// NewChecker returns a Checker that accepts timestamps within tolerance of the
// current time, inclusive. It fails if tolerance is not positive.
func NewChecker(tolerance time.Duration, opts ...Option) (*Checker, error) {
	if tolerance <= 0 {
		return nil, fmt.Errorf("replay: tolerance must be positive, got %s", tolerance)
	}
	c := &Checker{tolerance: tolerance, now: time.Now}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Check validates a timestamp header value. It returns ErrMissing, ErrInvalid,
// ErrTooOld or ErrTooNew; a nil result means the timestamp is within the
// tolerance.
func (c *Checker) Check(timestamp string) error {
	ts, err := ParseTimestamp(timestamp)
	if err != nil {
		return err
	}
	now := c.now()
	// Sub saturates instead of overflowing, so absurd timestamps are still
	// classified correctly. The future case therefore compares ts.Sub(now)
	// rather than negating now.Sub(ts), which would overflow for the most
	// negative duration.
	if age := now.Sub(ts); age > c.tolerance {
		return fmt.Errorf("%w: %s older than the %s tolerance", ErrTooOld, age-c.tolerance, c.tolerance)
	}
	if ahead := ts.Sub(now); ahead > c.tolerance {
		return fmt.Errorf("%w: %s beyond the %s tolerance", ErrTooNew, ahead-c.tolerance, c.tolerance)
	}
	return nil
}

// ParseTimestamp converts a decimal Unix timestamp in seconds to a time.Time.
// Signs, spaces, fractions and exponents are all rejected.
func ParseTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, ErrMissing
	}
	if len(s) > maxDigits {
		return time.Time{}, fmt.Errorf("%w: too many digits", ErrInvalid)
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return time.Time{}, fmt.Errorf("%w: unexpected character %q", ErrInvalid, s[i])
		}
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return time.Unix(secs, 0).UTC(), nil
}
