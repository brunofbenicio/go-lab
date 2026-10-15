// Package base62 encodes unsigned integers as short, URL-safe strings made of
// the digits 0-9, the uppercase letters A-Z and the lowercase letters a-z.
//
// Eleven characters are enough to represent any uint64, which makes the
// encoding a compact way to turn a number (a counter, a random value) into a
// short code for a URL.
package base62

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Alphabet lists the digits of the encoding in ascending value order.
const Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const (
	base = uint64(len(Alphabet))
	// maxDigits is the number of base62 digits needed for math.MaxUint64.
	maxDigits = 11
	// invalidDigit marks bytes outside the alphabet in decodeTable.
	invalidDigit = 0xFF
)

var (
	// ErrEmpty is returned when decoding an empty string.
	ErrEmpty = errors.New("base62: empty input")
	// ErrInvalidChar is returned when the input contains a byte that is not in
	// Alphabet.
	ErrInvalidChar = errors.New("base62: invalid character")
	// ErrOverflow is returned when the decoded value does not fit in a uint64.
	ErrOverflow = errors.New("base62: value overflows uint64")
)

// decodeTable maps every byte to its digit value, or invalidDigit.
var decodeTable = func() [256]byte {
	var t [256]byte
	for i := range t {
		t[i] = invalidDigit
	}
	for i := 0; i < len(Alphabet); i++ {
		t[Alphabet[i]] = byte(i)
	}
	return t
}()

// Encode returns the base62 representation of n without leading zeros.
// Zero is encoded as "0".
func Encode(n uint64) string {
	return EncodePadded(n, 0)
}

// EncodePadded returns the base62 representation of n left-padded with "0" to
// at least width characters. A width smaller than the natural length of n
// leaves the result untouched; it never truncates.
func EncodePadded(n uint64, width int) string {
	var buf [maxDigits]byte
	i := len(buf)
	for {
		i--
		buf[i] = Alphabet[n%base]
		n /= base
		if n == 0 {
			break
		}
	}
	digits := buf[i:]
	if len(digits) >= width {
		return string(digits)
	}

	var sb strings.Builder
	sb.Grow(width)
	for range width - len(digits) {
		sb.WriteByte(Alphabet[0])
	}
	sb.Write(digits)
	return sb.String()
}

// Decode parses a base62 string produced by Encode or EncodePadded. Leading
// zeros are accepted, so Decode("007") and Decode("7") return the same value.
//
// It returns ErrEmpty for an empty string, ErrInvalidChar (wrapped with the
// offending byte and its position) for characters outside Alphabet, and
// ErrOverflow when the value exceeds math.MaxUint64.
func Decode(s string) (uint64, error) {
	if s == "" {
		return 0, ErrEmpty
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		d := decodeTable[s[i]]
		if d == invalidDigit {
			return 0, fmt.Errorf("%w %q at position %d", ErrInvalidChar, s[i], i)
		}
		if n > (math.MaxUint64-uint64(d))/base {
			return 0, ErrOverflow
		}
		n = n*base + uint64(d)
	}
	return n, nil
}

// IsValid reports whether s is non-empty and made only of characters from
// Alphabet. It does not check whether the value fits in a uint64.
func IsValid(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if decodeTable[s[i]] == invalidDigit {
			return false
		}
	}
	return true
}
