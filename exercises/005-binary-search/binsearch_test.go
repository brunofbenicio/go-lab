package binsearch

import (
	"math/rand"
	"slices"
	"sort"
	"testing"
)

func TestSearchInts(t *testing.T) {
	xs := []int{1, 3, 3, 3, 7, 9, 12}
	tests := []struct {
		name      string
		target    int
		wantIndex int
		wantFound bool
	}{
		{"first element", 1, 0, true},
		{"duplicates return first", 3, 1, true},
		{"middle", 7, 4, true},
		{"last element", 12, 6, true},
		{"absent between", 8, 5, false},
		{"smaller than all", -4, 0, false},
		{"larger than all", 100, 7, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, ok := Search(xs, tt.target)
			if i != tt.wantIndex || ok != tt.wantFound {
				t.Errorf("Search(%v, %d) = (%d, %v), want (%d, %v)",
					xs, tt.target, i, ok, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestSearchStringsAndEmpty(t *testing.T) {
	words := []string{"apple", "banana", "cherry"}
	if i, ok := Search(words, "banana"); i != 1 || !ok {
		t.Errorf("Search(banana) = (%d, %v), want (1, true)", i, ok)
	}
	if i, ok := Search(words, "blueberry"); i != 2 || ok {
		t.Errorf("Search(blueberry) = (%d, %v), want (2, false)", i, ok)
	}
	if i, ok := Search([]float64(nil), 1.5); i != 0 || ok {
		t.Errorf("Search(nil) = (%d, %v), want (0, false)", i, ok)
	}
}

func TestBoundsAndCount(t *testing.T) {
	xs := []int{2, 2, 4, 4, 4, 8}
	tests := []struct {
		target, lower, upper, count int
	}{
		{1, 0, 0, 0},
		{2, 0, 2, 2},
		{3, 2, 2, 0},
		{4, 2, 5, 3},
		{8, 5, 6, 1},
		{9, 6, 6, 0},
	}
	for _, tt := range tests {
		if got := LowerBound(xs, tt.target); got != tt.lower {
			t.Errorf("LowerBound(%d) = %d, want %d", tt.target, got, tt.lower)
		}
		if got := UpperBound(xs, tt.target); got != tt.upper {
			t.Errorf("UpperBound(%d) = %d, want %d", tt.target, got, tt.upper)
		}
		if got := Count(xs, tt.target); got != tt.count {
			t.Errorf("Count(%d) = %d, want %d", tt.target, got, tt.count)
		}
	}
}

func TestMatchesStandardLibrary(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for round := 0; round < 200; round++ {
		xs := make([]int, rng.Intn(40))
		for i := range xs {
			xs[i] = rng.Intn(30)
		}
		slices.Sort(xs)
		target := rng.Intn(34) - 2
		if got, want := LowerBound(xs, target), sort.SearchInts(xs, target); got != want {
			t.Fatalf("LowerBound(%v, %d) = %d, want %d", xs, target, got, want)
		}
	}
}
