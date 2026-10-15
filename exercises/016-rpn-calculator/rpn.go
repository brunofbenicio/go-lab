// Package rpn evaluates arithmetic expressions written in Reverse Polish
// Notation (postfix), such as "3 4 + 2 *".
package rpn

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Errors returned by Eval. They are wrapped with context, so use errors.Is.
var (
	ErrEmpty          = errors.New("rpn: empty expression")
	ErrDivisionByZero = errors.New("rpn: division by zero")
	ErrStackUnderflow = errors.New("rpn: not enough operands")
	ErrLeftover       = errors.New("rpn: expression leaves more than one value")
	ErrToken          = errors.New("rpn: invalid token")
)

var binary = map[string]func(a, b float64) (float64, error){
	"+": func(a, b float64) (float64, error) { return a + b, nil },
	"-": func(a, b float64) (float64, error) { return a - b, nil },
	"*": func(a, b float64) (float64, error) { return a * b, nil },
	"/": func(a, b float64) (float64, error) {
		if b == 0 {
			return 0, ErrDivisionByZero
		}
		return a / b, nil
	},
	"%": func(a, b float64) (float64, error) {
		if b == 0 {
			return 0, ErrDivisionByZero
		}
		return math.Mod(a, b), nil
	},
	"^": func(a, b float64) (float64, error) { return math.Pow(a, b), nil },
}

var unary = map[string]func(a float64) float64{
	"neg":  func(a float64) float64 { return -a },
	"sqrt": math.Sqrt,
	"abs":  math.Abs,
}

// Eval evaluates a whitespace-separated postfix expression. Operands are
// decimal numbers; binary operators are + - * / % ^ and unary operators are
// neg, sqrt and abs.
func Eval(expr string) (float64, error) {
	tokens := strings.Fields(expr)
	if len(tokens) == 0 {
		return 0, ErrEmpty
	}
	var stack []float64
	for _, tok := range tokens {
		if op, ok := binary[tok]; ok {
			if len(stack) < 2 {
				return 0, fmt.Errorf("%w: %q needs 2, have %d", ErrStackUnderflow, tok, len(stack))
			}
			a, b := stack[len(stack)-2], stack[len(stack)-1]
			res, err := op(a, b)
			if err != nil {
				return 0, fmt.Errorf("evaluating %v %s %v: %w", a, tok, b, err)
			}
			stack = append(stack[:len(stack)-2], res)
			continue
		}
		if op, ok := unary[tok]; ok {
			if len(stack) < 1 {
				return 0, fmt.Errorf("%w: %q needs 1, have 0", ErrStackUnderflow, tok)
			}
			stack[len(stack)-1] = op(stack[len(stack)-1])
			continue
		}
		n, err := strconv.ParseFloat(tok, 64)
		// ParseFloat also accepts "inf" and "nan"; those are not numbers here.
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, fmt.Errorf("%w: %q", ErrToken, tok)
		}
		stack = append(stack, n)
	}
	if len(stack) != 1 {
		return 0, fmt.Errorf("%w: %d values remain", ErrLeftover, len(stack))
	}
	return stack[0], nil
}
