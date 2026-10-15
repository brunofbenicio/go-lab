// Package spiral reads a matrix in clockwise spiral order and builds matrices
// filled in that order.
package spiral

import "errors"

// ErrRagged is returned when the rows of a matrix have different lengths.
var ErrRagged = errors.New("spiral: rows have different lengths")

// Spiral returns the elements of m in clockwise spiral order, starting at the
// top-left corner. An empty matrix yields nil; rows of unequal length yield
// ErrRagged.
func Spiral(m [][]int) ([]int, error) {
	if len(m) == 0 {
		return nil, nil
	}
	cols := len(m[0])
	for _, row := range m {
		if len(row) != cols {
			return nil, ErrRagged
		}
	}

	out := make([]int, 0, len(m)*cols)
	top, bottom, left, right := 0, len(m)-1, 0, cols-1
	for top <= bottom && left <= right {
		for c := left; c <= right; c++ { // left to right along the top
			out = append(out, m[top][c])
		}
		top++
		for r := top; r <= bottom; r++ { // downwards along the right
			out = append(out, m[r][right])
		}
		right--
		if top <= bottom { // a single remaining row must not be walked twice
			for c := right; c >= left; c-- {
				out = append(out, m[bottom][c])
			}
			bottom--
		}
		if left <= right { // same for a single remaining column
			for r := bottom; r >= top; r-- {
				out = append(out, m[r][left])
			}
			left++
		}
	}
	return out, nil
}

// Fill returns a rows x cols matrix containing 1..rows*cols written in
// clockwise spiral order. It returns nil if either dimension is not positive.
func Fill(rows, cols int) [][]int {
	if rows <= 0 || cols <= 0 {
		return nil
	}
	m := make([][]int, rows)
	for i := range m {
		m[i] = make([]int, cols)
	}
	// Directions: right, down, left, up.
	dr := [4]int{0, 1, 0, -1}
	dc := [4]int{1, 0, -1, 0}
	r, c, d := 0, 0, 0
	for n := 1; n <= rows*cols; n++ {
		m[r][c] = n
		nr, nc := r+dr[d], c+dc[d]
		if nr < 0 || nr >= rows || nc < 0 || nc >= cols || m[nr][nc] != 0 {
			d = (d + 1) % 4 // hit the border or a filled cell: turn clockwise
			nr, nc = r+dr[d], c+dc[d]
		}
		r, c = nr, nc
	}
	return m
}
