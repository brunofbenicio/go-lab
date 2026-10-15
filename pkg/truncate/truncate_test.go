package truncate

import (
	"testing"
	"unicode/utf8"
)

func TestString(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		max    int
		suffix string
		want   string
	}{
		{"fits", "hello", 5, "...", "hello"},
		{"cut with suffix", "hello world", 8, "...", "hello..."},
		{"no suffix", "hello world", 5, "", "hello"},
		{"counts runes, not bytes", "ação é boa", 6, "…", "ação …"},
		{"emoji is not split", "go 🚀🚀🚀 now", 6, "…", "go 🚀🚀…"},
		{"suffix longer than max", "hello world", 2, "...", ".."},
		{"max zero", "hello", 0, "", ""},
		{"negative max", "hello", -1, "...", ""},
		{"empty input", "", 3, "...", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := String(tt.in, tt.max, tt.suffix)
			if got != tt.want {
				t.Errorf("String(%q, %d, %q) = %q, want %q", tt.in, tt.max, tt.suffix, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result %q is not valid UTF-8", got)
			}
		})
	}
}

func TestWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"fits", "short text", 20, "short text"},
		{"cuts at the last whole word", "the quick brown fox", 15, "the quick…"},
		{"cut exactly before a space", "the quick brown fox", 10, "the quick…"},
		{"long word is cut hard", "supercalifragilistic word", 8, "superca…"},
		{"trailing spaces are trimmed", "ab   cd efg", 7, "ab…"},
		{"tiny max falls back to String", "hello world", 1, "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Words(tt.in, tt.max, "…"); got != tt.want {
				t.Errorf("Words(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func TestMiddle(t *testing.T) {
	tests := []struct {
		in       string
		max      int
		ellipsis string
		want     string
	}{
		{"report-final.pdf", 20, "…", "report-final.pdf"},
		{"report-final.pdf", 9, "…", "repo….pdf"},
		{"abcdefghij", 7, "…", "abc…hij"},
		{"abcdefghij", 6, "…", "abc…ij"},
		{"abcdefghij", 7, "...", "ab...ij"},
		{"abcdefghij", 2, "...", "ab"},
		{"ação_final.txt", 9, "…", "ação….txt"},
		{"abc", -1, "…", ""},
	}
	for _, tt := range tests {
		if got := Middle(tt.in, tt.max, tt.ellipsis); got != tt.want {
			t.Errorf("Middle(%q, %d, %q) = %q, want %q", tt.in, tt.max, tt.ellipsis, got, tt.want)
		}
	}
}

func TestBytes(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel"},
		{"ação", 2, "a"},  // "ç" starts at byte 1 and needs 2 bytes
		{"ação", 3, "aç"}, // exactly on a rune boundary
		{"ação", 4, "aç"}, // would split "ã"
		{"🚀", 3, ""},      // 4-byte rune does not fit at all
		{"🚀a", 4, "🚀"},
		{"hello", 0, ""},
		{"hello", -2, ""},
	}
	for _, tt := range tests {
		got := Bytes(tt.in, tt.max)
		if got != tt.want || !utf8.ValidString(got) {
			t.Errorf("Bytes(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}
