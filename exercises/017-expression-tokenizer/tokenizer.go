// Package tokenizer splits arithmetic expressions such as
// "max(x1, 2.5e-3) * (y + 4)" into tokens with their positions.
package tokenizer

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Kind classifies a token.
type Kind int

// Token kinds.
const (
	Number Kind = iota + 1
	Ident
	Operator
	LParen
	RParen
	Comma
)

var kindNames = map[Kind]string{
	Number: "Number", Ident: "Ident", Operator: "Operator",
	LParen: "LParen", RParen: "RParen", Comma: "Comma",
}

// String returns the name of the kind.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Token is a lexical element of the source text.
type Token struct {
	Kind Kind
	Text string
	Pos  int // byte offset of the first character in the source
}

// SyntaxError is returned for text that cannot be tokenized.
type SyntaxError struct {
	Pos int
	Msg string
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("tokenizer: position %d: %s", e.Pos, e.Msg)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// scanNumber returns the end offset of the number starting at src[i]. It
// accepts "12", "3.14", ".5" and exponents like "1e10" or "2.5E-3".
func scanNumber(src string, i int) (int, error) {
	skipDigits := func(j int) int {
		for j < len(src) && isDigit(src[j]) {
			j++
		}
		return j
	}
	j := skipDigits(i)
	if j < len(src) && src[j] == '.' {
		k := skipDigits(j + 1)
		if k == j+1 {
			return 0, &SyntaxError{j, "digit expected after decimal point"}
		}
		j = k
	}
	if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
		k := j + 1
		if k < len(src) && (src[k] == '+' || src[k] == '-') {
			k++
		}
		e := skipDigits(k)
		if e == k {
			return 0, &SyntaxError{j, "digit expected in exponent"}
		}
		j = e
	}
	return j, nil
}

// Tokenize splits src into tokens, skipping whitespace. Identifiers start with
// a letter or underscore and may contain digits. Operators are + - * / ^ %.
// It returns a *SyntaxError for unexpected characters or malformed numbers.
func Tokenize(src string) ([]Token, error) {
	var tokens []Token
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRuneInString(src[i:])
		switch {
		case unicode.IsSpace(r):
			i += size
		case isDigit(src[i]) || (src[i] == '.' && i+1 < len(src) && isDigit(src[i+1])):
			end, err := scanNumber(src, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{Number, src[i:end], i})
			i = end
		case unicode.IsLetter(r) || r == '_':
			end := i + size
			for end < len(src) {
				next, n := utf8.DecodeRuneInString(src[end:])
				if !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_' {
					break
				}
				end += n
			}
			tokens = append(tokens, Token{Ident, src[i:end], i})
			i = end
		default:
			kind, ok := single[src[i]]
			if !ok {
				return nil, &SyntaxError{i, fmt.Sprintf("unexpected character %q", r)}
			}
			tokens = append(tokens, Token{kind, src[i : i+1], i})
			i++
		}
	}
	return tokens, nil
}

// single maps one-character tokens to their kind.
var single = map[byte]Kind{
	'+': Operator, '-': Operator, '*': Operator, '/': Operator, '^': Operator, '%': Operator,
	'(': LParen, ')': RParen, ',': Comma,
}
