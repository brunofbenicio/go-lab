// Package levenshtein computes the edit distance between strings and uses it
// to suggest close matches ("did you mean ...?").
package levenshtein

import (
	"cmp"
	"slices"
)

// Distance returns the minimum number of single-rune insertions, deletions and
// substitutions needed to turn a into b. It keeps only two rows of the DP
// table, so memory use is O(min-sized row) instead of O(len(a)*len(b)).
func Distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	prev := make([]int, len(y)+1)
	curr := make([]int, len(y)+1)
	for j := range prev {
		prev[j] = j // turning "" into y[:j] takes j insertions
	}
	for i := 1; i <= len(x); i++ {
		curr[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,      // delete x[i-1]
				curr[j-1]+1,    // insert y[j-1]
				prev[j-1]+cost, // substitute (or keep)
			)
		}
		prev, curr = curr, prev
	}
	return prev[len(y)]
}

// Similarity returns a score between 0 (nothing in common) and 1 (equal
// strings) derived from the edit distance and the longer length.
func Similarity(a, b string) float64 {
	longest := max(len([]rune(a)), len([]rune(b)))
	if longest == 0 {
		return 1
	}
	return 1 - float64(Distance(a, b))/float64(longest)
}

// Suggest returns the candidates whose distance to word is at most maxDist,
// closest first. Ties are broken alphabetically.
func Suggest(word string, candidates []string, maxDist int) []string {
	type scored struct {
		text string
		dist int
	}
	var found []scored
	for _, c := range candidates {
		if d := Distance(word, c); d <= maxDist {
			found = append(found, scored{c, d})
		}
	}
	slices.SortFunc(found, func(p, q scored) int {
		return cmp.Or(cmp.Compare(p.dist, q.dist), cmp.Compare(p.text, q.text))
	})
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.text
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
