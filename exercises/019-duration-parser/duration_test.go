package duration

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Duration
		wantErr error
	}{
		{"seconds", "45s", 45 * time.Second, nil},
		{"hours and minutes", "1h30m", 90 * time.Minute, nil},
		{"day", "1d", 24 * time.Hour, nil},
		{"weeks", "2w", 14 * Day, nil},
		{"all units", "1w2d3h4m5s6ms", Week + 2*Day + 3*time.Hour + 4*time.Minute + 5*time.Second + 6*time.Millisecond, nil},
		{"fraction", "1.5h", 90 * time.Minute, nil},
		{"fraction without integer part", ".5s", 500 * time.Millisecond, nil},
		{"milliseconds", "250ms", 250 * time.Millisecond, nil},
		{"microseconds ascii", "100us", 100 * time.Microsecond, nil},
		{"microseconds greek mu", "100µs", 100 * time.Microsecond, nil},
		{"nanoseconds", "7ns", 7, nil},
		{"minutes then milliseconds", "1m30s500ms", 90*time.Second + 500*time.Millisecond, nil},
		{"explicit plus sign", "+5s", 5 * time.Second, nil},
		{"negative", "-1h30m", -90 * time.Minute, nil},
		{"bare zero", "0", 0, nil},
		{"zero with unit", "0s", 0, nil},
		{"repeated units add up", "1h1h", 2 * time.Hour, nil},
		{"max int64", "9223372036854775807ns", math.MaxInt64, nil},
		{"min int64", "-9223372036854775808ns", math.MinInt64, nil},
		{"empty", "", 0, ErrSyntax},
		{"only sign", "-", 0, ErrSyntax},
		{"unit without number", "h", 0, ErrSyntax},
		{"number without unit", "10", 0, ErrSyntax},
		{"unknown unit", "5x", 0, ErrUnknownUnit},
		{"unit is case sensitive", "5H", 0, ErrUnknownUnit},
		{"sign in the middle", "1h-30m", 0, ErrSyntax},
		{"double dot", "1..5h", 0, ErrSyntax},
		{"overflow weeks", "99999999999w", 0, ErrOverflow},
		{"overflow by summing", "9223372036854775807ns1ns", 0, ErrOverflow},
		{"negative overflow", "-9223372036854775809ns", 0, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Parse(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{90 * time.Minute, "1h30m"},
		{26 * time.Hour, "1d2h"},
		{8 * Day, "1w1d"},
		{1500 * time.Millisecond, "1s500ms"},
		{-90 * time.Second, "-1m30s"},
		{time.Nanosecond, "1ns"},
		{Week + 2*Day + 3*time.Hour + 4*time.Minute + 5*time.Second, "1w2d3h4m5s"},
	}
	for _, tt := range tests {
		if got := Format(tt.d); got != tt.want {
			t.Errorf("Format(%d) = %q, want %q", int64(tt.d), got, tt.want)
		}
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	values := []time.Duration{
		0, 1, 999, 1001, time.Second, 61 * time.Second, 25*time.Hour + 1,
		400 * Day, -Week, math.MaxInt64, math.MinInt64,
	}
	for _, d := range values {
		s := Format(d)
		got, err := Parse(s)
		if err != nil || got != d {
			t.Errorf("Parse(Format(%d)) = (%v, %v) via %q", int64(d), got, err, s)
		}
	}
}
