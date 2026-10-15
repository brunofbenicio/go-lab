// Package sudoku parses and validates Sudoku boards.
package sudoku

import (
	"errors"
	"fmt"
	"strings"
)

// Board is a 9x9 Sudoku grid. Zero means an empty cell.
type Board [9][9]int

// ErrFormat is returned for malformed board text or out-of-range digits.
var ErrFormat = errors.New("sudoku: invalid board")

// ConflictError reports a digit that appears twice in a row, column or box.
type ConflictError struct {
	Unit  string // "row", "column" or "box"
	Index int    // 1-based index of the unit
	Digit int
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	return fmt.Sprintf("sudoku: digit %d repeated in %s %d", e.Digit, e.Unit, e.Index)
}

// Parse reads a board from 81 characters, ignoring whitespace so that boards
// can be written over nine lines. Digits 1-9 are givens; '0' and '.' are
// empty cells.
func Parse(s string) (Board, error) {
	var b Board
	clean := strings.Join(strings.Fields(s), "")
	if len(clean) != 81 {
		return b, fmt.Errorf("%w: expected 81 cells, got %d", ErrFormat, len(clean))
	}
	for i := 0; i < 81; i++ {
		ch := clean[i]
		switch {
		case ch >= '1' && ch <= '9':
			b[i/9][i%9] = int(ch - '0')
		case ch == '0' || ch == '.':
		default:
			return b, fmt.Errorf("%w: unexpected character %q", ErrFormat, ch)
		}
	}
	return b, nil
}

// Validate checks that no digit repeats in any row, column or 3x3 box. Empty
// cells are ignored, so partially filled boards can be validated. It returns
// nil, an ErrFormat error, or a *ConflictError for the first conflict found
// (rows first, then columns, then boxes).
func (b Board) Validate() error {
	for r := range b {
		for c := range b[r] {
			if b[r][c] < 0 || b[r][c] > 9 {
				return fmt.Errorf("%w: cell (%d,%d) holds %d", ErrFormat, r+1, c+1, b[r][c])
			}
		}
	}
	units := []struct {
		name string
		cell func(unit, k int) int // k-th cell of the given unit
	}{
		{"row", func(u, k int) int { return b[u][k] }},
		{"column", func(u, k int) int { return b[k][u] }},
		{"box", func(u, k int) int { return b[u/3*3+k/3][u%3*3+k%3] }},
	}
	for _, unit := range units {
		for u := 0; u < 9; u++ {
			var seen uint16
			for k := 0; k < 9; k++ {
				d := unit.cell(u, k)
				if d == 0 {
					continue
				}
				if seen&(1<<d) != 0 {
					return &ConflictError{Unit: unit.name, Index: u + 1, Digit: d}
				}
				seen |= 1 << d
			}
		}
	}
	return nil
}

// Solved reports whether the board is completely filled and valid.
func (b Board) Solved() bool {
	for _, row := range b {
		for _, d := range row {
			if d == 0 {
				return false
			}
		}
	}
	return b.Validate() == nil
}
