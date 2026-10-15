// Package money represents monetary amounts as integer minor units (cents),
// avoiding the rounding errors of floating point. Arithmetic is checked for
// overflow and refuses to mix currencies, and Allocate splits an amount
// without losing or inventing a single cent.
//
// Amounts always have two decimal places.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

// Errors returned by the package.
var (
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	ErrOverflow         = errors.New("money: arithmetic overflow")
	ErrInvalidAmount    = errors.New("money: invalid amount")
	ErrInvalidRatios    = errors.New("money: ratios must be non-negative with a positive sum")
)

// Money is an amount of a currency, stored in cents. The zero value is an
// empty amount without a currency.
type Money struct {
	cents    int64
	currency string
}

// New creates a Money from an amount in cents.
func New(cents int64, currency string) Money { return Money{cents, currency} }

// Parse reads a decimal amount such as "12.34", "-0.5" or "10", with at most
// two decimal places.
func Parse(amount, currency string) (Money, error) {
	digits, neg := strings.CutPrefix(amount, "-")
	whole, frac, _ := strings.Cut(digits, ".")
	if whole == "" || len(frac) > 2 || !allDigits(whole) || !allDigits(frac) ||
		strings.Contains(digits, ".") && frac == "" {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
	}
	text := whole + frac + strings.Repeat("0", 2-len(frac))
	if neg {
		text = "-" + text
	}
	cents, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
	}
	return Money{cents, currency}, nil
}

// Cents returns the amount in cents.
func (m Money) Cents() int64 { return m.cents }

// Currency returns the currency code given at creation.
func (m Money) Currency() string { return m.currency }

// Add returns m + o.
func (m Money) Add(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if o.cents > 0 && m.cents > math.MaxInt64-o.cents || o.cents < 0 && m.cents < math.MinInt64-o.cents {
		return Money{}, ErrOverflow
	}
	return Money{m.cents + o.cents, m.currency}, nil
}

// Sub returns m - o.
func (m Money) Sub(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if o.cents < 0 && m.cents > math.MaxInt64+o.cents || o.cents > 0 && m.cents < math.MinInt64+o.cents {
		return Money{}, ErrOverflow
	}
	return Money{m.cents - o.cents, m.currency}, nil
}

// Mul returns m multiplied by the integer n.
func (m Money) Mul(n int64) (Money, error) {
	if m.cents == 0 || n == 0 {
		return Money{0, m.currency}, nil
	}
	p := m.cents * n
	if p/n != m.cents || m.cents == math.MinInt64 && n == -1 {
		return Money{}, ErrOverflow
	}
	return Money{p, m.currency}, nil
}

// Compare returns -1, 0 or +1 if m is less than, equal to or greater than o.
func (m Money) Compare(o Money) (int, error) {
	if m.currency != o.currency {
		return 0, ErrCurrencyMismatch
	}
	switch {
	case m.cents < o.cents:
		return -1, nil
	case m.cents > o.cents:
		return 1, nil
	}
	return 0, nil
}

// Allocate splits m proportionally to ratios. The parts always add up to
// exactly m: leftover cents go one at a time to the first non-zero ratios, so
// 100 split 1:1:1 gives 34, 33 and 33. Ratios must be non-negative with a
// positive sum.
func (m Money) Allocate(ratios ...int) ([]Money, error) {
	var total uint64
	for _, r := range ratios {
		if r < 0 || total+uint64(r) < total {
			return nil, ErrInvalidRatios
		}
		total += uint64(r)
	}
	if total == 0 {
		return nil, ErrInvalidRatios
	}

	abs := uint64(m.cents)
	if m.cents < 0 {
		abs = -abs
	}
	shares := make([]uint64, len(ratios))
	var used uint64
	for i, r := range ratios {
		// abs*r can exceed 64 bits, so use a 128-bit product; the quotient
		// is at most abs because r <= total.
		hi, lo := bits.Mul64(abs, uint64(r))
		shares[i], _ = bits.Div64(hi, lo, total)
		used += shares[i]
	}
	for i, rem := 0, abs-used; rem > 0; i = (i + 1) % len(ratios) {
		if ratios[i] > 0 {
			shares[i]++
			rem--
		}
	}

	parts := make([]Money, len(shares))
	for i, s := range shares {
		cents := int64(s)
		if m.cents < 0 {
			cents = -cents
		}
		parts[i] = Money{cents, m.currency}
	}
	return parts, nil
}

// String formats the amount as "12.34 BRL".
func (m Money) String() string {
	abs, sign := uint64(m.cents), ""
	if m.cents < 0 {
		abs, sign = -abs, "-"
	}
	return fmt.Sprintf("%s%d.%02d %s", sign, abs/100, abs%100, m.currency)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
