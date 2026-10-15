// Package humanize formats numbers for people: byte sizes in SI (kB, MB) and
// IEC (KiB, MiB) units, compact durations such as "1d 12h", and integers with
// thousands separators.
package humanize

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var (
	siUnits  = []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}
	iecUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
)

// Bytes formats n using decimal (SI) units, base 1000: 1500 -> "1.5 kB".
func Bytes(n int64) string { return formatBytes(n, 1000, siUnits) }

// IBytes formats n using binary (IEC) units, base 1024: 1536 -> "1.5 KiB".
func IBytes(n int64) string { return formatBytes(n, 1024, iecUnits) }

func formatBytes(n int64, base float64, units []string) string {
	v := math.Abs(float64(n))
	if v < base {
		return fmt.Sprintf("%d B", n)
	}
	i := 0
	for v >= base && i < len(units)-1 {
		v /= base
		i++
	}
	r := math.Round(v*10) / 10
	if r >= base && i < len(units)-1 { // 999.96 kB rounds up to 1 MB
		r, i = 1, i+1
	}
	sign := ""
	if n < 0 {
		sign = "-"
	}
	return sign + strings.TrimSuffix(strconv.FormatFloat(r, 'f', 1, 64), ".0") + " " + units[i]
}

// Duration formats d with its non-zero units from days down to seconds, for
// example "2h 30m 15s" or "1d 12h". Durations of one second or more drop the
// sub-second part; shorter ones are shown in milliseconds ("250ms") or in
// time.Duration's own notation when under a millisecond.
func Duration(d time.Duration) string {
	neg := d < 0
	u := uint64(d)
	if neg {
		u = -u // two's complement: also correct for the minimum Duration
	}
	if u < uint64(time.Second) {
		if u >= uint64(time.Millisecond) {
			return fmt.Sprintf("%s%dms", sign(neg), u/uint64(time.Millisecond))
		}
		return d.String()
	}
	secs := u / uint64(time.Second)
	var parts []string
	for _, unit := range []struct {
		name string
		size uint64
	}{{"d", 86400}, {"h", 3600}, {"m", 60}, {"s", 1}} {
		if n := secs / unit.size; n > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", n, unit.name))
			secs %= unit.size
		}
	}
	return sign(neg) + strings.Join(parts, " ")
}

// Thousands formats n with sep between groups of three digits:
// Thousands(1234567, ".") -> "1.234.567".
func Thousands(n int64, sep string) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}

func sign(neg bool) string {
	if neg {
		return "-"
	}
	return ""
}
