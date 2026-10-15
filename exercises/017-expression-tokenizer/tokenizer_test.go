package tokenizer

import (
	"errors"
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Token
	}{
		{"empty", "", nil},
		{"only whitespace", " \t\n", nil},
		{"precedence example", "1 + 2*3", []Token{
			{Number, "1", 0}, {Operator, "+", 2}, {Number, "2", 4},
			{Operator, "*", 5}, {Number, "3", 6},
		}},
		{"function call", "max(x1, 2)", []Token{
			{Ident, "max", 0}, {LParen, "(", 3}, {Ident, "x1", 4},
			{Comma, ",", 6}, {Number, "2", 8}, {RParen, ")", 9},
		}},
		{"decimals and exponents", "3.14 .5 1e10 2.5E-3", []Token{
			{Number, "3.14", 0}, {Number, ".5", 5}, {Number, "1e10", 8}, {Number, "2.5E-3", 13},
		}},
		{"unary minus is an operator", "-4", []Token{
			{Operator, "-", 0}, {Number, "4", 1},
		}},
		{"underscores and unicode names", "_tmp*área", []Token{
			{Ident, "_tmp", 0}, {Operator, "*", 4}, {Ident, "área", 5},
		}},
		{"adjacent tokens", "(a)(b)", []Token{
			{LParen, "(", 0}, {Ident, "a", 1}, {RParen, ")", 2},
			{LParen, "(", 3}, {Ident, "b", 4}, {RParen, ")", 5},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tokenize(tt.src)
			if err != nil {
				t.Fatalf("Tokenize(%q) unexpected error: %v", tt.src, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tokenize(%q) =\n%v\nwant\n%v", tt.src, got, tt.want)
			}
		})
	}
}

func TestTokenizeErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantPos int
	}{
		{"unexpected character", "1 + $", 4},
		{"trailing decimal point", "1.", 1},
		{"lonely dot", "a . b", 2},
		{"empty exponent", "2e", 1},
		{"signed empty exponent", "1 + 3e+", 5},
		{"semicolon", "a;b", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Tokenize(tt.src)
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("Tokenize(%q) error = %v, want *SyntaxError", tt.src, err)
			}
			if se.Pos != tt.wantPos {
				t.Errorf("Tokenize(%q) error at %d, want %d (%v)", tt.src, se.Pos, tt.wantPos, se)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{Number, "Number"}, {Ident, "Ident"}, {Operator, "Operator"},
		{LParen, "LParen"}, {RParen, "RParen"}, {Comma, "Comma"}, {Kind(99), "Kind(99)"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(tt.kind), got, tt.want)
		}
	}
}
