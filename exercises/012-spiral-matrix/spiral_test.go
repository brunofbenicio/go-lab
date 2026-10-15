package spiral

import (
	"errors"
	"reflect"
	"testing"
)

func TestSpiral(t *testing.T) {
	tests := []struct {
		name string
		in   [][]int
		want []int
	}{
		{"empty", nil, nil},
		{"single cell", [][]int{{7}}, []int{7}},
		{"3x3", [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, []int{1, 2, 3, 6, 9, 8, 7, 4, 5}},
		{"3x4", [][]int{{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}},
			[]int{1, 2, 3, 4, 8, 12, 11, 10, 9, 5, 6, 7}},
		{"single row", [][]int{{1, 2, 3}}, []int{1, 2, 3}},
		{"single column", [][]int{{1}, {2}, {3}}, []int{1, 2, 3}},
		{"4x1 column then row", [][]int{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
			[]int{1, 2, 4, 6, 8, 7, 5, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Spiral(tt.in)
			if err != nil {
				t.Fatalf("Spiral(%v) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Spiral(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSpiralRagged(t *testing.T) {
	if _, err := Spiral([][]int{{1, 2}, {3}}); !errors.Is(err, ErrRagged) {
		t.Errorf("Spiral() error = %v, want ErrRagged", err)
	}
}

func TestFill(t *testing.T) {
	tests := []struct {
		name       string
		rows, cols int
		want       [][]int
	}{
		{"zero rows", 0, 3, nil},
		{"negative cols", 2, -1, nil},
		{"1x1", 1, 1, [][]int{{1}}},
		{"3x3", 3, 3, [][]int{{1, 2, 3}, {8, 9, 4}, {7, 6, 5}}},
		{"2x4", 2, 4, [][]int{{1, 2, 3, 4}, {8, 7, 6, 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Fill(tt.rows, tt.cols); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Fill(%d, %d) = %v, want %v", tt.rows, tt.cols, got, tt.want)
			}
		})
	}
}

// Reading a filled matrix in spiral order must give back 1..n.
func TestSpiralOfFillIsSequential(t *testing.T) {
	for rows := 1; rows <= 7; rows++ {
		for cols := 1; cols <= 7; cols++ {
			got, err := Spiral(Fill(rows, cols))
			if err != nil {
				t.Fatalf("%dx%d: %v", rows, cols, err)
			}
			for i, v := range got {
				if v != i+1 {
					t.Fatalf("%dx%d: element %d = %d, want %d", rows, cols, i, v, i+1)
				}
			}
			if len(got) != rows*cols {
				t.Fatalf("%dx%d: got %d elements, want %d", rows, cols, len(got), rows*cols)
			}
		}
	}
}
