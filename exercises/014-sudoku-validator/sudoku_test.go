package sudoku

import (
	"errors"
	"testing"
)

const partial = `
53..7....
6..195...
.98....6.
8...6...3
4..8.3..1
7...2...6
.6....28.
...419..5
....8..79`

const solved = `
534678912
672195348
198342567
859761423
426853791
713924856
961537284
287419635
345286179`

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"partial board", partial, false},
		{"solved board", solved, false},
		{"zeros as empty", "0" + partial[2:], false},
		{"too short", "123", true},
		{"too long", solved + "1", true},
		{"invalid character", "x" + solved[2:], true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrFormat) {
				t.Errorf("Parse() error = %v, want ErrFormat", err)
			}
		})
	}
}

func TestValidateAndSolved(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Board)
		board      string
		wantUnit   string
		wantIndex  int
		wantDigit  int
		wantSolved bool
	}{
		{name: "partial board is valid", board: partial},
		{name: "complete board is solved", board: solved, wantSolved: true},
		{
			name: "repeated in a row", board: partial,
			mutate:   func(b *Board) { b[0][2] = 5 },
			wantUnit: "row", wantIndex: 1, wantDigit: 5,
		},
		{
			name: "repeated in a column", board: partial,
			mutate:   func(b *Board) { b[4][0] = 0; b[7][0] = 6 }, // 6 already at (2,1)
			wantUnit: "column", wantIndex: 1, wantDigit: 6,
		},
		{
			name: "full board with a wrong digit", board: solved,
			mutate:   func(b *Board) { b[0][0], b[1][1] = 1, 1 },
			wantUnit: "row", wantIndex: 1, wantDigit: 1,
		},
		{
			// (1,3) and (3,1) share a box but no row or column.
			name: "repeated only in a box", board: partial,
			mutate:   func(b *Board) { b[0][2] = 2; b[2][0] = 2 },
			wantUnit: "box", wantIndex: 1, wantDigit: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(tt.board)
			if err != nil {
				t.Fatal(err)
			}
			if tt.mutate != nil {
				tt.mutate(&b)
			}
			err = b.Validate()
			if tt.wantUnit == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				if got := b.Solved(); got != tt.wantSolved {
					t.Errorf("Solved() = %v, want %v", got, tt.wantSolved)
				}
				return
			}
			var ce *ConflictError
			if !errors.As(err, &ce) {
				t.Fatalf("Validate() = %v, want *ConflictError", err)
			}
			if ce.Unit != tt.wantUnit || ce.Index != tt.wantIndex || ce.Digit != tt.wantDigit {
				t.Errorf("Validate() = %+v, want %s %d digit %d", *ce, tt.wantUnit, tt.wantIndex, tt.wantDigit)
			}
			if b.Solved() {
				t.Error("Solved() = true for an invalid board")
			}
		})
	}
}

func TestValidateRejectsOutOfRangeDigits(t *testing.T) {
	var b Board
	b[3][4] = 10
	if err := b.Validate(); !errors.Is(err, ErrFormat) {
		t.Errorf("Validate() = %v, want ErrFormat", err)
	}
}

func TestEmptyBoardIsValidButNotSolved(t *testing.T) {
	var b Board
	if err := b.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if b.Solved() {
		t.Error("Solved() = true for an empty board")
	}
}
