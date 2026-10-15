// Package idlist formats slices of integers as delimited text ("1,2,3"), the
// kind of helper used to build query strings or log messages. It provides a
// straightforward version and an optimized one, so the benchmarks have
// something meaningful to compare.
package idlist

import (
	"strconv"
	"strings"
)

// JoinNaive builds the result by repeated string concatenation. Every += copies
// the whole string so far, making it quadratic in the number of items.
func JoinNaive(ids []int, sep string) string {
	s := ""
	for i, id := range ids {
		if i > 0 {
			s += sep
		}
		s += strconv.Itoa(id)
	}
	return s
}

// JoinBuilder uses a strings.Builder, growing its buffer as needed.
func JoinBuilder(ids []int, sep string) string {
	var b strings.Builder
	for i, id := range ids {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(strconv.Itoa(id))
	}
	return b.String()
}

// Join appends digits straight into one pre-sized byte slice, avoiding the
// temporary string that strconv.Itoa allocates, and converts once at the end.
func Join(ids []int, sep string) string {
	if len(ids) == 0 {
		return ""
	}
	// Assume about 8 bytes per number. It is only an estimate: append still
	// grows the slice if some numbers are longer, so the result stays correct.
	buf := make([]byte, 0, len(ids)*(len(sep)+8))
	for i, id := range ids {
		if i > 0 {
			buf = append(buf, sep...)
		}
		buf = strconv.AppendInt(buf, int64(id), 10)
	}
	return string(buf)
}
