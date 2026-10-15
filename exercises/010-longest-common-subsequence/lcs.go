// Package lcs computes the longest common subsequence of two strings with
// dynamic programming.
package lcs

// table returns the DP table where t[i][j] is the length of the LCS of
// x[i:] and y[j:]. Its dimensions are (len(x)+1) by (len(y)+1).
func table(x, y []rune) [][]int {
	t := make([][]int, len(x)+1)
	for i := range t {
		t[i] = make([]int, len(y)+1)
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else {
				t[i][j] = max(t[i+1][j], t[i][j+1])
			}
		}
	}
	return t
}

// Length returns the length (in runes) of the longest common subsequence of a
// and b.
func Length(a, b string) int {
	return table([]rune(a), []rune(b))[0][0]
}

// LCS returns one longest common subsequence of a and b. A subsequence keeps
// the relative order of its characters but need not be contiguous. When
// several answers exist, the one that favors earlier characters of a is
// returned.
func LCS(a, b string) string {
	x, y := []rune(a), []rune(b)
	t := table(x, y)

	var out []rune
	for i, j := 0, 0; i < len(x) && j < len(y); {
		switch {
		case x[i] == y[j]:
			out = append(out, x[i])
			i++
			j++
		case t[i+1][j] >= t[i][j+1]:
			i++
		default:
			j++
		}
	}
	return string(out)
}
