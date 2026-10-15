package bytesize

import (
	"errors"
	"testing"
)

// TestParse uses the classic table-driven layout: a slice of anonymous
// structs, one row per scenario, executed by a single loop.
func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr error // nil means success is expected
	}{
		{name: "plain bytes", input: "512", want: 512},
		{name: "explicit byte unit", input: "512B", want: 512},
		{name: "SI kilobytes", input: "10KB", want: 10_000},
		{name: "IEC kibibytes", input: "10KiB", want: 10_240},
		{name: "case insensitive", input: "2mb", want: 2_000_000},
		{name: "space before unit", input: "1.5 GiB", want: 1_610_612_736},
		{name: "surrounding spaces", input: "  3 KB  ", want: 3_000},
		{name: "fraction is rounded", input: "1.6B", want: 2},
		{name: "empty", input: "", wantErr: ErrInvalid},
		{name: "only spaces", input: "   ", wantErr: ErrInvalid},
		{name: "unknown unit", input: "10XB", wantErr: ErrInvalid},
		{name: "missing number", input: "KB", wantErr: ErrInvalid},
		{name: "two dots", input: "1.2.3KB", wantErr: ErrInvalid},
		{name: "negative is rejected", input: "-5KB", wantErr: ErrInvalid},
		{name: "overflow", input: "99999999999GiB", wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)

			// Compare errors with errors.Is, never by message text.
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Parse(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestFormat shows the map-based variant: the map key is the test name, which
// keeps rows compact when there are only an input and an expected output.
// Map iteration order is random, which also helps catch tests that depend on
// execution order.
func TestFormat(t *testing.T) {
	tests := map[string]struct {
		in   int64
		want string
	}{
		"zero":                {0, "0 B"},
		"just below a kilo":   {999, "999 B"},
		"exactly one kilo":    {1000, "1.0 KB"},
		"one and a half kilo": {1500, "1.5 KB"},
		"megabyte":            {1_000_000, "1.0 MB"},
		"gigabyte":            {2_500_000_000, "2.5 GB"},
		"largest int64":       {9_223_372_036_854_775_807, "9.2 EB"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Format(tt.in); got != tt.want {
				t.Errorf("Format(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
