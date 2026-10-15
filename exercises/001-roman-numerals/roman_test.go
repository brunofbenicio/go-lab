package roman

import (
	"errors"
	"testing"
)

func TestToRoman(t *testing.T) {
	tests := []struct {
		name    string
		in      int
		want    string
		wantErr error
	}{
		{"one", 1, "I", nil},
		{"four", 4, "IV", nil},
		{"nine", 9, "IX", nil},
		{"fifty-eight", 58, "LVIII", nil},
		{"ninety-four", 94, "XCIV", nil},
		{"year 1994", 1994, "MCMXCIV", nil},
		{"year 2024", 2024, "MMXXIV", nil},
		{"maximum", 3999, "MMMCMXCIX", nil},
		{"zero", 0, "", ErrOutOfRange},
		{"negative", -5, "", ErrOutOfRange},
		{"too large", 4000, "", ErrOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToRoman(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ToRoman(%d) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ToRoman(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFromRoman(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int
		wantErr error
	}{
		{"simple", "III", 3, nil},
		{"subtractive", "IV", 4, nil},
		{"complex", "MCMXCIV", 1994, nil},
		{"maximum", "MMMCMXCIX", 3999, nil},
		{"empty", "", 0, ErrInvalid},
		{"lower case", "iv", 0, ErrInvalid},
		{"unknown symbol", "ABC", 0, ErrInvalid},
		{"four repeated", "IIII", 0, ErrInvalid},
		{"invalid subtraction", "VX", 0, ErrInvalid},
		{"skipped order", "IC", 0, ErrInvalid},
		{"too many thousands", "MMMM", 0, ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromRoman(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("FromRoman(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("FromRoman(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	for n := 1; n <= 3999; n++ {
		s, err := ToRoman(n)
		if err != nil {
			t.Fatalf("ToRoman(%d): %v", n, err)
		}
		back, err := FromRoman(s)
		if err != nil || back != n {
			t.Fatalf("FromRoman(%q) = %d, %v; want %d", s, back, err, n)
		}
	}
}
