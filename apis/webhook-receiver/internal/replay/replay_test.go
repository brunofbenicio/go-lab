package replay

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

func unix(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

func newChecker(t *testing.T, tolerance time.Duration, now time.Time) *Checker {
	t.Helper()
	c, err := NewChecker(tolerance, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewChecker() failed: %v", err)
	}
	return c
}

func TestNewChecker(t *testing.T) {
	tests := []struct {
		name      string
		tolerance time.Duration
		wantErr   bool
	}{
		{"default tolerance", DefaultTolerance, false},
		{"one second", time.Second, false},
		{"zero", 0, true},
		{"negative", -time.Minute, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewChecker(tt.tolerance)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewChecker() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && c == nil {
				t.Error("NewChecker() returned nil without an error")
			}
		})
	}
}

func TestCheck(t *testing.T) {
	const tolerance = 5 * time.Minute

	tests := []struct {
		name      string
		timestamp string
		wantErr   error
	}{
		{"exactly now", unix(t0), nil},
		{"a second ago", unix(t0.Add(-time.Second)), nil},
		{"a minute ahead", unix(t0.Add(time.Minute)), nil},
		{"exactly at the past limit", unix(t0.Add(-tolerance)), nil},
		{"exactly at the future limit", unix(t0.Add(tolerance)), nil},
		{"one second beyond the past limit", unix(t0.Add(-tolerance - time.Second)), ErrTooOld},
		{"one second beyond the future limit", unix(t0.Add(tolerance + time.Second)), ErrTooNew},
		{"a day old", unix(t0.Add(-24 * time.Hour)), ErrTooOld},
		{"the Unix epoch", "0", ErrTooOld},
		{"a year ahead", unix(t0.AddDate(1, 0, 0)), ErrTooNew},

		{"empty", "", ErrMissing},
		{"words", "yesterday", ErrInvalid},
		{"negative", "-1740830400", ErrInvalid},
		{"explicit plus sign", "+1740830400", ErrInvalid},
		{"fraction", "1740830400.5", ErrInvalid},
		{"exponent", "1.7408304e9", ErrInvalid},
		{"milliseconds", "1740830400000", ErrInvalid},
		{"leading space", " 1740830400", ErrInvalid},
		{"trailing newline", "1740830400\n", ErrInvalid},
		{"hexadecimal", "0x67c2f1c0", ErrInvalid},
		{"non-ASCII digits", "١٧٤٠٨٣٠٤٠٠", ErrInvalid},
		{"twelve digits is the longest accepted", "999999999999", ErrTooNew},
		{"thirteen digits", "9999999999999", ErrInvalid},
	}
	c := newChecker(t, tolerance, t0)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.Check(tt.timestamp); !errors.Is(err, tt.wantErr) {
				t.Errorf("Check(%q) error = %v, want %v", tt.timestamp, err, tt.wantErr)
			}
		})
	}
}

func TestCheckFollowsTheClock(t *testing.T) {
	now := t0
	c, err := NewChecker(time.Minute, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	stamp := unix(t0)

	if err := c.Check(stamp); err != nil {
		t.Fatalf("fresh timestamp rejected: %v", err)
	}
	now = t0.Add(time.Minute)
	if err := c.Check(stamp); err != nil {
		t.Errorf("timestamp at the limit rejected: %v", err)
	}
	now = t0.Add(time.Minute + time.Second)
	if err := c.Check(stamp); !errors.Is(err, ErrTooOld) {
		t.Errorf("stale timestamp: error = %v, want ErrTooOld", err)
	}
	now = t0.Add(-time.Minute - time.Second)
	if err := c.Check(stamp); !errors.Is(err, ErrTooNew) {
		t.Errorf("timestamp from the future: error = %v, want ErrTooNew", err)
	}
}

func TestCheckIgnoresSubSecondClockPrecision(t *testing.T) {
	// The clock is half a second past the timestamp's second: skew is 5m0.5s,
	// which is beyond a 5 minute tolerance.
	c := newChecker(t, 5*time.Minute, t0.Add(500*time.Millisecond))
	if err := c.Check(unix(t0.Add(-5 * time.Minute))); !errors.Is(err, ErrTooOld) {
		t.Errorf("error = %v, want ErrTooOld", err)
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Time
		wantErr error
	}{
		{"1740830400", t0, nil},
		{"0", time.Unix(0, 0).UTC(), nil},
		{"007", time.Unix(7, 0).UTC(), nil},
		{"", time.Time{}, ErrMissing},
		{"12a", time.Time{}, ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseTimestamp(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseTimestamp(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseTimestamp(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestErrorMessagesExplainTheSkew(t *testing.T) {
	c := newChecker(t, time.Minute, t0)

	err := c.Check(unix(t0.Add(-90 * time.Second)))
	if err == nil || err.Error() != "replay: timestamp is too old: 30s older than the 1m0s tolerance" {
		t.Errorf("too old: %v", err)
	}
	err = c.Check(unix(t0.Add(2 * time.Minute)))
	if err == nil || err.Error() != "replay: timestamp is too far in the future: 1m0s beyond the 1m0s tolerance" {
		t.Errorf("too new: %v", err)
	}
}
