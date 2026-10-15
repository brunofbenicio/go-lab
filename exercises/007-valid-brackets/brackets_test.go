package brackets

import (
	"errors"
	"testing"
)

func TestValid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", true},
		{"simple pair", "()", true},
		{"all kinds", "()[]{}", true},
		{"nested", "{[()]}", true},
		{"with text", "func(a[i], map[string]int{1: 2})", true},
		{"no brackets", "just text", true},
		{"interleaved", "([)]", false},
		{"unclosed", "((", false},
		{"unopened", "())", false},
		{"only closer", "}", false},
		{"wrong closer", "(]", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.in); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCheckReportsPosition(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantPos int
	}{
		{"unexpected closer", "ab)", 2},
		{"mismatch points at closer", "a(b]c", 3},
		{"unclosed points at opener", "x[(y)", 1},
		{"innermost unclosed", "{[", 1},
		{"multibyte prefix counts bytes", "é)", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.in)
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("Check(%q) error = %v, want *SyntaxError", tt.in, err)
			}
			if se.Pos != tt.wantPos {
				t.Errorf("Check(%q) position = %d, want %d (%v)", tt.in, se.Pos, tt.wantPos, se)
			}
		})
	}
}

func TestCheckValidInputReturnsNil(t *testing.T) {
	if err := Check("[{()}]"); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
}
