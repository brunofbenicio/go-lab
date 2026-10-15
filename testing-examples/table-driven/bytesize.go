// Package bytesize converts human-readable sizes such as "512", "10KB" or
// "1.5 GiB" into a number of bytes, and formats byte counts back into text.
package bytesize

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Sentinel errors returned (wrapped) by Parse.
var (
	// ErrInvalid means the input is not a well-formed size.
	ErrInvalid = errors.New("bytesize: invalid size")
	// ErrOverflow means the size does not fit in an int64.
	ErrOverflow = errors.New("bytesize: size overflows int64")
)

// multipliers maps an upper-cased unit to its size in bytes. SI units
// (KB, MB, GB) use powers of 1000; IEC units (KiB, MiB, GiB) use powers of 1024.
var multipliers = map[string]float64{
	"": 1, "B": 1,
	"KB": 1e3, "MB": 1e6, "GB": 1e9,
	"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30,
}

// Parse converts text like "10KB" or "1.5 GiB" into bytes. Units are
// case-insensitive and may be separated from the number by spaces.
// Fractional results are rounded to the nearest byte.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("%w: empty input", ErrInvalid)
	}
	end := strings.IndexFunc(s, func(r rune) bool {
		return !unicode.IsDigit(r) && r != '.'
	})
	if end < 0 {
		end = len(s)
	}
	number, unit := s[:end], strings.ToUpper(strings.TrimSpace(s[end:]))

	mult, ok := multipliers[unit]
	if !ok {
		return 0, fmt.Errorf("%w: unknown unit %q", ErrInvalid, unit)
	}
	n, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bad number %q", ErrInvalid, number)
	}
	bytes := math.Round(n * mult)
	if bytes >= math.MaxInt64 {
		return 0, ErrOverflow
	}
	return int64(bytes), nil
}

// Format renders n using SI units with one decimal place, e.g. "1.5 KB".
func Format(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
