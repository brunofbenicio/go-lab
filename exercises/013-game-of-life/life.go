// Package life implements Conway's Game of Life on a rectangular grid that can
// either have dead borders or wrap around like a torus.
package life

import (
	"errors"
	"fmt"
	"strings"
)

// Grid is a rectangular board of cells. The zero value is not usable; create
// grids with New or Parse.
type Grid struct {
	rows, cols int
	wrap       bool
	cells      []bool
}

// New returns an empty (all dead) grid. When wrap is true the edges connect to
// the opposite side; otherwise cells outside the grid are always dead.
func New(rows, cols int, wrap bool) (*Grid, error) {
	if rows <= 0 || cols <= 0 {
		return nil, fmt.Errorf("life: invalid size %dx%d", rows, cols)
	}
	return &Grid{rows: rows, cols: cols, wrap: wrap, cells: make([]bool, rows*cols)}, nil
}

// Parse builds a grid from text lines where '#' is a live cell and '.' is a
// dead one. All lines must have the same length.
func Parse(wrap bool, lines ...string) (*Grid, error) {
	if len(lines) == 0 {
		return nil, errors.New("life: no lines")
	}
	g, err := New(len(lines), len(lines[0]), wrap)
	if err != nil {
		return nil, err
	}
	for r, line := range lines {
		if len(line) != g.cols {
			return nil, fmt.Errorf("life: line %d has %d columns, want %d", r, len(line), g.cols)
		}
		for c := 0; c < len(line); c++ {
			switch line[c] {
			case '#':
				g.Set(r, c, true)
			case '.':
			default:
				return nil, fmt.Errorf("life: invalid character %q at line %d, column %d", line[c], r, c)
			}
		}
	}
	return g, nil
}

// Alive reports whether the cell at (r, c) is alive. Cells outside the grid
// are reported dead.
func (g *Grid) Alive(r, c int) bool {
	if r < 0 || r >= g.rows || c < 0 || c >= g.cols {
		return false
	}
	return g.cells[r*g.cols+c]
}

// Set marks the cell at (r, c) alive or dead. Coordinates outside the grid are
// ignored.
func (g *Grid) Set(r, c int, alive bool) {
	if r < 0 || r >= g.rows || c < 0 || c >= g.cols {
		return
	}
	g.cells[r*g.cols+c] = alive
}

// Population returns the number of live cells.
func (g *Grid) Population() int {
	n := 0
	for _, alive := range g.cells {
		if alive {
			n++
		}
	}
	return n
}

// neighbors counts the live cells around (r, c).
func (g *Grid) neighbors(r, c int) int {
	n := 0
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			rr, cc := r+dr, c+dc
			if g.wrap {
				rr = (rr + g.rows) % g.rows
				cc = (cc + g.cols) % g.cols
			}
			if g.Alive(rr, cc) {
				n++
			}
		}
	}
	return n
}

// Step returns the next state of the board, leaving g unchanged. A live cell survives
// with two or three live neighbors; a dead cell is born with exactly three.
func (g *Grid) Step() *Grid {
	next := &Grid{rows: g.rows, cols: g.cols, wrap: g.wrap, cells: make([]bool, len(g.cells))}
	for r := 0; r < g.rows; r++ {
		for c := 0; c < g.cols; c++ {
			n := g.neighbors(r, c)
			next.cells[r*g.cols+c] = n == 3 || (n == 2 && g.Alive(r, c))
		}
	}
	return next
}

// Equal reports whether two grids have the same size and live cells.
func (g *Grid) Equal(o *Grid) bool {
	if g.rows != o.rows || g.cols != o.cols {
		return false
	}
	for i, alive := range g.cells {
		if alive != o.cells[i] {
			return false
		}
	}
	return true
}

// String renders the grid in the same format accepted by Parse, one line per
// row without a trailing newline.
func (g *Grid) String() string {
	var sb strings.Builder
	for r := 0; r < g.rows; r++ {
		if r > 0 {
			sb.WriteByte('\n')
		}
		for c := 0; c < g.cols; c++ {
			if g.Alive(r, c) {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
	}
	return sb.String()
}
