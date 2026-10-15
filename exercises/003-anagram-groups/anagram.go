// Package anagram groups words that are anagrams of each other.
package anagram

import (
	"slices"
	"strings"
	"unicode"
)

// Key returns a canonical signature shared by all anagrams of word: its
// letters and digits, lower-cased and sorted. Spaces and punctuation are
// ignored, so "dirty room" and "dormitory" have the same key.
func Key(word string) string {
	runes := make([]rune, 0, len(word))
	for _, r := range strings.ToLower(word) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			runes = append(runes, r)
		}
	}
	slices.Sort(runes)
	return string(runes)
}

// AreAnagrams reports whether a and b are anagrams of each other. Identical
// words (ignoring case) are not considered anagrams.
func AreAnagrams(a, b string) bool {
	return !strings.EqualFold(a, b) && Key(a) == Key(b)
}

// Group splits words into groups of anagrams. Groups appear in the order of
// their first member, and words keep their input order inside each group.
// Words are returned exactly as given (original case and spelling).
func Group(words []string) [][]string {
	index := make(map[string]int)
	var groups [][]string
	for _, w := range words {
		k := Key(w)
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], w)
	}
	return groups
}
