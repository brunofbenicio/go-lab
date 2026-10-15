// Package stringcase converts identifiers between naming conventions:
// camelCase, PascalCase, snake_case, kebab-case and SCREAMING_SNAKE_CASE.
//
// Words are detected from separators (spaces, underscores, hyphens, any
// punctuation) and from case changes, with acronym awareness: "HTTPServer"
// is the words "HTTP" and "Server". Digits stay attached to the word before
// them. Unicode letters are supported.
package stringcase

import (
	"strings"
	"unicode"
)

// Words splits s into its words, preserving their original case.
func Words(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if unicode.IsUpper(r) && len(cur) > 0 {
			prevUpper := unicode.IsUpper(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			// "fooBar" splits before B; "HTTPServer" splits before S.
			if !prevUpper || nextLower {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

// Snake converts s to snake_case.
func Snake(s string) string { return joinLower(s, "_") }

// Kebab converts s to kebab-case.
func Kebab(s string) string { return joinLower(s, "-") }

// ScreamingSnake converts s to SCREAMING_SNAKE_CASE.
func ScreamingSnake(s string) string { return strings.ToUpper(Snake(s)) }

// Pascal converts s to PascalCase.
func Pascal(s string) string {
	var b strings.Builder
	for _, w := range Words(s) {
		b.WriteString(capitalize(w))
	}
	return b.String()
}

// Camel converts s to camelCase.
func Camel(s string) string {
	words := Words(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.ToLower(words[0]))
	for _, w := range words[1:] {
		b.WriteString(capitalize(w))
	}
	return b.String()
}

func joinLower(s, sep string) string {
	words := Words(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, sep)
}

// capitalize upper-cases the first rune of w and lower-cases the rest.
func capitalize(w string) string {
	r := []rune(strings.ToLower(w))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
