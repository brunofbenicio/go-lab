// Package duration parses and formats human-friendly durations such as
// "1w2d3h", "90m" or "1.5h". It extends time.ParseDuration with days and
// weeks.
package duration

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"time"
)

// Units that time.Duration does not define itself.
const (
	Day  = 24 * time.Hour
	Week = 7 * Day
)

// Errors returned by Parse; they are wrapped with the offending input.
var (
	ErrSyntax      = errors.New("duration: invalid syntax")
	ErrUnknownUnit = errors.New("duration: unknown unit")
	ErrOverflow    = errors.New("duration: value out of range")
)

var units = map[string]time.Duration{
	"ns": time.Nanosecond,
	"us": time.Microsecond,
	"µs": time.Microsecond,
	"ms": time.Millisecond,
	"s":  time.Second,
	"m":  time.Minute,
	"h":  time.Hour,
	"d":  Day,
	"w":  Week,
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// takeDigits splits s into its leading run of digits and the rest.
func takeDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// Parse converts text such as "1d2h30m", "-45s" or "1.5h" into a Duration.
// A duration is a sequence of <number><unit> pairs, optionally preceded by a
// sign. Units are ns, us (or µs), ms, s, m, h, d and w. Fractions are rounded
// to the nearest nanosecond. The bare string "0" is accepted without a unit.
func Parse(s string) (time.Duration, error) {
	orig := s
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("%w: %q", ErrSyntax, orig)
	}
	if s == "0" {
		return 0, nil
	}

	var total uint64 // magnitude, so that math.MinInt64 can be represented
	for s != "" {
		intPart, rest := takeDigits(s)
		var frac string
		if strings.HasPrefix(rest, ".") {
			frac, rest = takeDigits(rest[1:])
		}
		if intPart == "" && frac == "" {
			return 0, fmt.Errorf("%w: expected a number in %q", ErrSyntax, orig)
		}
		// The unit name runs until the next number, decimal point or sign.
		k := 0
		for k < len(rest) && !isDigit(rest[k]) && !strings.ContainsRune(".+-", rune(rest[k])) {
			k++
		}
		name := rest[:k]
		s = rest[k:]
		if name == "" {
			return 0, fmt.Errorf("%w: missing unit in %q", ErrSyntax, orig)
		}
		unit, ok := units[name]
		if !ok {
			return 0, fmt.Errorf("%w %q in %q", ErrUnknownUnit, name, orig)
		}

		part, err := scale(intPart, frac, uint64(unit))
		if err != nil {
			return 0, fmt.Errorf("%w: %q", err, orig)
		}
		var carry uint64
		if total, carry = bits.Add64(total, part, 0); carry != 0 {
			return 0, fmt.Errorf("%w: %q", ErrOverflow, orig)
		}
	}

	if neg {
		if total > 1<<63 {
			return 0, fmt.Errorf("%w: %q", ErrOverflow, orig)
		}
		return time.Duration(-total), nil
	}
	if total > math.MaxInt64 {
		return 0, fmt.Errorf("%w: %q", ErrOverflow, orig)
	}
	return time.Duration(total), nil
}

// scale returns (intPart.frac) * unit as an integer number of nanoseconds.
func scale(intPart, frac string, unit uint64) (uint64, error) {
	var v uint64
	if intPart != "" {
		n, err := strconv.ParseUint(intPart, 10, 64)
		if err != nil {
			return 0, ErrOverflow
		}
		hi, lo := bits.Mul64(n, unit)
		if hi != 0 {
			return 0, ErrOverflow
		}
		v = lo
	}
	if frac != "" {
		f, err := strconv.ParseFloat("0."+frac, 64)
		if err != nil {
			return 0, ErrSyntax
		}
		v += uint64(math.Round(f * float64(unit)))
	}
	return v, nil
}

// Format renders d with the largest units first, for example "1w2d3h4m5s" or
// "1s500ms". Zero is "0s". Parse(Format(d)) == d for every d.
func Format(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	mag := uint64(d)
	var sb strings.Builder
	if d < 0 {
		sb.WriteByte('-')
		mag = -mag // two's complement negation also works for math.MinInt64
	}
	steps := []struct {
		name string
		size uint64
	}{
		{"w", uint64(Week)}, {"d", uint64(Day)}, {"h", uint64(time.Hour)},
		{"m", uint64(time.Minute)}, {"s", uint64(time.Second)},
		{"ms", uint64(time.Millisecond)}, {"us", uint64(time.Microsecond)},
		{"ns", uint64(time.Nanosecond)},
	}
	for _, st := range steps {
		if n := mag / st.size; n > 0 {
			sb.WriteString(strconv.FormatUint(n, 10))
			sb.WriteString(st.name)
			mag -= n * st.size
		}
	}
	return sb.String()
}
