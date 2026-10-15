package levenshtein

import (
	"math"
	"reflect"
	"testing"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"both empty", "", "", 0},
		{"insert all", "", "abc", 3},
		{"delete all", "abc", "", 3},
		{"equal", "golang", "golang", 0},
		{"classic", "kitten", "sitting", 3},
		{"flaw lawn", "flaw", "lawn", 2},
		{"intention execution", "intention", "execution", 5},
		{"single substitution", "cat", "cut", 1},
		{"unicode counts runes", "ação", "acao", 2},
		{"case sensitive", "Go", "go", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Distance(tt.a, tt.b); got != tt.want {
				t.Errorf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
			if got := Distance(tt.b, tt.a); got != tt.want {
				t.Errorf("Distance(%q, %q) = %d, want %d (symmetry)", tt.b, tt.a, got, tt.want)
			}
		})
	}
}

func TestSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want float64
	}{
		{"both empty", "", "", 1},
		{"equal", "abc", "abc", 1},
		{"disjoint", "abc", "xyz", 0},
		{"one of four differs", "abcd", "abce", 0.75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Similarity(tt.a, tt.b); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Similarity(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSuggest(t *testing.T) {
	commands := []string{"commit", "checkout", "branch", "status", "stash", "merge", "rebase"}
	tests := []struct {
		name    string
		word    string
		maxDist int
		want    []string
	}{
		{"typo", "comit", 2, []string{"commit"}},
		{"closest first, then alphabetical", "stat", 3, []string{"stash", "status"}},
		{"exact match", "merge", 0, []string{"merge"}},
		{"nothing close", "zzzzzzzz", 2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Suggest(tt.word, commands, tt.maxDist); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Suggest(%q, _, %d) = %v, want %v", tt.word, tt.maxDist, got, tt.want)
			}
		})
	}
}
