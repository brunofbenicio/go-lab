// Package wordfreq counts word frequencies in text. Its functions are pure and
// hold no shared state, so their tests can safely run in parallel.
package wordfreq

import (
	"sort"
	"strings"
	"unicode"
)

// WordCount is a word and how many times it appeared.
type WordCount struct {
	Word  string
	Count int
}

// Count returns how many times each lower-cased word appears in text. Words
// are maximal runs of letters and digits; everything else is a separator.
func Count(text string) map[string]int {
	counts := make(map[string]int)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, w := range words {
		counts[strings.ToLower(w)]++
	}
	return counts
}

// Top returns the n most frequent words, ordered by count (descending) and
// then alphabetically so the result is deterministic. If n is larger than the
// number of distinct words, all of them are returned.
func Top(counts map[string]int, n int) []WordCount {
	all := make([]WordCount, 0, len(counts))
	for w, c := range counts {
		all = append(all, WordCount{Word: w, Count: c})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Count != all[j].Count {
			return all[i].Count > all[j].Count
		}
		return all[i].Word < all[j].Word
	})
	if n < len(all) {
		all = all[:max(n, 0)]
	}
	return all
}
