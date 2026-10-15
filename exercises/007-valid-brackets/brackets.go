// Package brackets validates balanced (), [] and {} in a string and reports
// exactly where the first problem is.
package brackets

import "fmt"

// SyntaxError describes the first unbalanced bracket found by Check.
type SyntaxError struct {
	Pos int    // byte offset of the offending character
	Msg string // human-readable explanation
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("brackets: position %d: %s", e.Pos, e.Msg)
}

// closers maps each closing bracket to its opening counterpart.
var closers = map[rune]rune{')': '(', ']': '[', '}': '{'}

// pair returns the closing bracket that matches the opening bracket open.
func pair(open rune) rune {
	switch open {
	case '(':
		return ')'
	case '[':
		return ']'
	default:
		return '}'
	}
}

type opening struct {
	char rune
	pos  int
}

// Check returns nil when every bracket in s is correctly opened, closed and
// nested. Characters other than brackets are ignored. Otherwise it returns a
// *SyntaxError pointing at the first problem.
func Check(s string) error {
	var stack []opening
	for i, r := range s {
		switch r {
		case '(', '[', '{':
			stack = append(stack, opening{r, i})
		case ')', ']', '}':
			if len(stack) == 0 {
				return &SyntaxError{i, fmt.Sprintf("unexpected %q with no matching opener", r)}
			}
			top := stack[len(stack)-1]
			if top.char != closers[r] {
				return &SyntaxError{i, fmt.Sprintf("found %q but expected %q", r, pair(top.char))}
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		top := stack[len(stack)-1]
		return &SyntaxError{top.pos, fmt.Sprintf("%q is never closed", top.char)}
	}
	return nil
}

// Valid reports whether the brackets in s are balanced.
func Valid(s string) bool {
	return Check(s) == nil
}
