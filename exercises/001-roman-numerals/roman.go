// Package roman converts integers to Roman numerals and back.
//
// Only values from 1 to 3999 are supported, and only canonical numerals are
// accepted when parsing (for example "IV" is valid but "IIII" is not).
package roman

import (
	"errors"
	"fmt"
	"strings"
)

// Errors returned by the conversions.
var (
	ErrOutOfRange = errors.New("roman: value out of range 1..3999")
	ErrInvalid    = errors.New("roman: invalid numeral")
)

// numerals lists the symbols in descending order, including the subtractive
// forms (CM, CD, XC, XL, IX, IV), which makes a greedy conversion correct.
var numerals = []struct {
	value  int
	symbol string
}{
	{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
	{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
	{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
}

// symbolValues maps each single symbol to its value.
var symbolValues = map[byte]int{
	'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000,
}

// ToRoman returns the Roman numeral for n, which must be in 1..3999.
func ToRoman(n int) (string, error) {
	if n < 1 || n > 3999 {
		return "", ErrOutOfRange
	}
	var sb strings.Builder
	for _, nu := range numerals {
		for n >= nu.value {
			sb.WriteString(nu.symbol)
			n -= nu.value
		}
	}
	return sb.String(), nil
}

// FromRoman parses an upper-case Roman numeral. Non-canonical spellings such
// as "IIII", "VX" or "IC" are rejected with ErrInvalid.
func FromRoman(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("%w: empty string", ErrInvalid)
	}
	total := 0
	for i := 0; i < len(s); i++ {
		v, ok := symbolValues[s[i]]
		if !ok {
			return 0, fmt.Errorf("%w: unexpected symbol %q at position %d", ErrInvalid, s[i], i)
		}
		// A smaller symbol before a larger one is subtracted (IV = 4).
		if i+1 < len(s) && v < symbolValues[s[i+1]] {
			total -= v
		} else {
			total += v
		}
	}
	// Converting back guarantees that only the canonical form is accepted.
	if canon, err := ToRoman(total); err != nil || canon != s {
		return 0, fmt.Errorf("%w: %q is not canonical", ErrInvalid, s)
	}
	return total, nil
}
