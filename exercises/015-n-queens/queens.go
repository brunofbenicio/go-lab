// Package queens solves the N-Queens puzzle for small boards using
// backtracking with bitmasks.
package queens

import (
	"errors"
	"math/bits"
	"strings"
)

// MaxN is the largest board size accepted, which keeps Solutions from
// building enormous result sets.
const MaxN = 12

// ErrSize is returned when n is outside 1..MaxN.
var ErrSize = errors.New("queens: board size must be between 1 and 12")

// Count returns how many ways n queens can be placed on an n x n board so
// that none attack each other.
func Count(n int) (int, error) {
	if n < 1 || n > MaxN {
		return 0, ErrSize
	}
	count := 0
	search(n, 0, 0, 0, 0, make([]int, n), func([]int) { count++ })
	return count, nil
}

// Solutions returns every placement as a slice where element r is the column
// of the queen in row r. Solutions are in lexicographic order.
func Solutions(n int) ([][]int, error) {
	if n < 1 || n > MaxN {
		return nil, ErrSize
	}
	var all [][]int
	search(n, 0, 0, 0, 0, make([]int, n), func(cur []int) {
		all = append(all, append([]int(nil), cur...))
	})
	return all, nil
}

// search places a queen in each row. cols, d1 and d2 are bitmasks of the
// columns and the two diagonal directions that are already attacked in the
// current row; visit is called with every complete placement.
func search(n, row int, cols, d1, d2 uint, cur []int, visit func([]int)) {
	if row == n {
		visit(cur)
		return
	}
	free := ^(cols | d1 | d2) & (1<<uint(n) - 1)
	for free != 0 {
		bit := free & -free // lowest free column
		free &^= bit
		cur[row] = bits.TrailingZeros(bit)
		search(n, row+1, cols|bit, (d1|bit)<<1, (d2|bit)>>1, cur, visit)
	}
}

// Render draws a placement as text, using 'Q' for queens and '.' for empty
// squares, one line per row.
func Render(solution []int) string {
	var sb strings.Builder
	for r, col := range solution {
		if r > 0 {
			sb.WriteByte('\n')
		}
		for c := range solution {
			if c == col {
				sb.WriteByte('Q')
			} else {
				sb.WriteByte('.')
			}
		}
	}
	return sb.String()
}
