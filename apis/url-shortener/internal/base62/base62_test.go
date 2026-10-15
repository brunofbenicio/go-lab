package base62

import (
	"errors"
	"math"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   uint64
		want string
	}{
		{"zero", 0, "0"},
		{"single digit", 9, "9"},
		{"first uppercase", 10, "A"},
		{"first lowercase", 36, "a"},
		{"last digit", 61, "z"},
		{"first two-digit value", 62, "10"},
		{"two digits", 3843, "zz"},
		{"three digits", 3844, "100"},
		{"large value", 1_000_000_000, "15ftgG"},
		{"max uint64", math.MaxUint64, "LygHa16AHYF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Encode(tt.in); got != tt.want {
				t.Errorf("Encode(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEncodePadded(t *testing.T) {
	tests := []struct {
		name  string
		in    uint64
		width int
		want  string
	}{
		{"zero padded", 0, 7, "0000000"},
		{"small value padded", 61, 4, "000z"},
		{"exact width", 3844, 3, "100"},
		{"width smaller than value never truncates", 3844, 1, "100"},
		{"zero width", 62, 0, "10"},
		{"negative width", 62, -5, "10"},
		{"max uint64 with wide padding", math.MaxUint64, 13, "00LygHa16AHYF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncodePadded(tt.in, tt.width); got != tt.want {
				t.Errorf("EncodePadded(%d, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    uint64
		wantErr error
	}{
		{"zero", "0", 0, nil},
		{"uppercase digit", "A", 10, nil},
		{"lowercase digit", "a", 36, nil},
		{"two digits", "10", 62, nil},
		{"leading zeros are accepted", "000z", 61, nil},
		{"max uint64", "LygHa16AHYF", math.MaxUint64, nil},
		{"empty", "", 0, ErrEmpty},
		{"dash", "ab-c", 0, ErrInvalidChar},
		{"space", "ab c", 0, ErrInvalidChar},
		{"non-ASCII", "aé", 0, ErrInvalidChar},
		{"one above max uint64", "LygHa16AHYG", 0, ErrOverflow},
		{"twelve digits", "100000000000", 0, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Decode(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Decode(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecodeErrorMentionsPosition(t *testing.T) {
	_, err := Decode("ab-c")
	if err == nil {
		t.Fatal("Decode(\"ab-c\") returned no error")
	}
	const want = `base62: invalid character '-' at position 2`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRoundTrip(t *testing.T) {
	values := []uint64{
		0, 1, 61, 62, 63, 3843, 3844, 238327, 238328,
		1 << 32, 1<<63 - 1, 1 << 63, math.MaxUint64 - 1, math.MaxUint64,
	}
	for _, v := range values {
		got, err := Decode(Encode(v))
		if err != nil {
			t.Fatalf("Decode(Encode(%d)) failed: %v", v, err)
		}
		if got != v {
			t.Errorf("Decode(Encode(%d)) = %d", v, got)
		}

		padded, err := Decode(EncodePadded(v, 14))
		if err != nil {
			t.Fatalf("Decode(EncodePadded(%d, 14)) failed: %v", v, err)
		}
		if padded != v {
			t.Errorf("Decode(EncodePadded(%d, 14)) = %d", v, padded)
		}
	}
}

func TestEncodePreservesOrderForEqualWidth(t *testing.T) {
	prev := EncodePadded(0, 6)
	for n := uint64(1); n < 5000; n += 7 {
		cur := EncodePadded(n, 6)
		if cur <= prev {
			t.Fatalf("EncodePadded(%d, 6) = %q is not greater than previous %q", n, cur, prev)
		}
		prev = cur
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"abc123", true},
		{"ABCxyz09", true},
		{"0", true},
		{"", false},
		{"abc-123", false},
		{"abc_123", false},
		{"abc 123", false},
		{"docs.", false},
		{"é", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := IsValid(tt.in); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
