package rpn

import (
	"errors"
	"math"
	"testing"
)

func TestEval(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    float64
		wantErr error
	}{
		{"single number", "42", 42, nil},
		{"addition", "3 4 +", 7, nil},
		{"order matters", "10 4 -", 6, nil},
		{"classic example", "5 1 2 + 4 * + 3 -", 14, nil},
		{"division", "7 2 /", 3.5, nil},
		{"power", "2 10 ^", 1024, nil},
		{"modulo", "17 5 %", 2, nil},
		{"decimals", "1.5 2.25 +", 3.75, nil},
		{"negative literal", "-3 4 *", -12, nil},
		{"unary operators", "9 sqrt 2 neg *", -6, nil},
		{"abs", "-7 abs", 7, nil},
		{"extra whitespace", "  2   3\t+ \n", 5, nil},
		{"empty", "", 0, ErrEmpty},
		{"only spaces", "   ", 0, ErrEmpty},
		{"division by zero", "1 0 /", 0, ErrDivisionByZero},
		{"modulo by zero", "1 0 %", 0, ErrDivisionByZero},
		{"missing operand", "1 +", 0, ErrStackUnderflow},
		{"operator first", "+", 0, ErrStackUnderflow},
		{"unary without operand", "sqrt", 0, ErrStackUnderflow},
		{"too many values", "1 2", 0, ErrLeftover},
		{"unknown token", "1 x +", 0, ErrToken},
		{"nan is rejected", "nan 1 +", 0, ErrToken},
		{"inf is rejected", "inf", 0, ErrToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Eval(%q) error = %v, want %v", tt.expr, err, tt.wantErr)
			}
			if tt.wantErr == nil && math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}
