// Package truncate shortens strings safely: it counts runes instead of bytes,
// never splits a multi-byte character, and offers word-boundary, middle and
// byte-budget variants.
//
// Runes are the unit, not grapheme clusters: a character built from a base
// plus combining marks may still be cut between its parts.
package truncate

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// String shortens s to at most max runes, including suffix, which is appended
// when something was cut. If suffix alone does not fit, its first max runes
// are returned. A negative max yields "".
func String(s string, max int, suffix string) string {
	if max < 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	room := max - utf8.RuneCountInString(suffix)
	if room < 0 {
		return string([]rune(suffix)[:max])
	}
	return string([]rune(s)[:room]) + suffix
}

// Words behaves like String but cuts at the last whole word that fits, so no
// word is chopped in half. A single word longer than the room is cut hard.
func Words(s string, max int, suffix string) string {
	if max < 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	room := max - utf8.RuneCountInString(suffix)
	if room <= 0 {
		return String(s, max, suffix)
	}
	head := runes[:room]
	if !unicode.IsSpace(runes[room]) { // the cut falls inside a word
		for i := len(head) - 1; i > 0; i-- {
			if unicode.IsSpace(head[i]) {
				head = head[:i]
				break
			}
		}
	}
	return strings.TrimRightFunc(string(head), unicode.IsSpace) + suffix
}

// Middle shortens s to at most max runes by replacing the middle with
// ellipsis, keeping both ends visible, which preserves things like file
// extensions: Middle("quarterly-report-final.pdf", 14, "…") is
// "quarter…al.pdf". When the number of kept runes is odd, the start gets the
// extra one.
func Middle(s string, max int, ellipsis string) string {
	if max < 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	keep := max - utf8.RuneCountInString(ellipsis)
	if keep <= 0 {
		return String(s, max, "")
	}
	head, tail := (keep+1)/2, keep/2
	return string(runes[:head]) + ellipsis + string(runes[len(runes)-tail:])
}

// Bytes cuts s to at most max bytes without breaking a UTF-8 sequence, which
// suits fixed-size fields such as database columns.
func Bytes(s string, max int) string {
	if max < 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
