package queens

import (
	"errors"
	"reflect"
	"testing"
)

func TestCount(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{1, 1}, {2, 0}, {3, 0}, {4, 2}, {5, 10}, {6, 4}, {7, 40}, {8, 92}, {9, 352},
	}
	for _, tt := range tests {
		got, err := Count(tt.n)
		if err != nil || got != tt.want {
			t.Errorf("Count(%d) = (%d, %v), want (%d, nil)", tt.n, got, err, tt.want)
		}
	}
}

func TestSizeLimits(t *testing.T) {
	for _, n := range []int{-1, 0, MaxN + 1} {
		if _, err := Count(n); !errors.Is(err, ErrSize) {
			t.Errorf("Count(%d) error = %v, want ErrSize", n, err)
		}
		if _, err := Solutions(n); !errors.Is(err, ErrSize) {
			t.Errorf("Solutions(%d) error = %v, want ErrSize", n, err)
		}
	}
}

func TestSolutionsFourQueens(t *testing.T) {
	got, err := Solutions(4)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{1, 3, 0, 2}, {2, 0, 3, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Solutions(4) = %v, want %v", got, want)
	}
}

func TestSolutionsAreValidAndDistinct(t *testing.T) {
	for n := 1; n <= 8; n++ {
		sols, err := Solutions(n)
		if err != nil {
			t.Fatal(err)
		}
		if count, _ := Count(n); len(sols) != count {
			t.Errorf("n=%d: Solutions has %d entries, Count says %d", n, len(sols), count)
		}
		seen := make(map[string]bool)
		for _, s := range sols {
			if !valid(s) {
				t.Fatalf("n=%d: placement %v has attacking queens", n, s)
			}
			key := Render(s)
			if seen[key] {
				t.Fatalf("n=%d: duplicate placement %v", n, s)
			}
			seen[key] = true
		}
	}
}

// valid checks every pair of queens directly, independently of the bitmasks.
func valid(s []int) bool {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[i] == s[j] || s[j]-s[i] == j-i || s[i]-s[j] == j-i {
				return false
			}
		}
	}
	return true
}

func TestRender(t *testing.T) {
	got := Render([]int{1, 3, 0, 2})
	want := ".Q..\n...Q\nQ...\n..Q."
	if got != want {
		t.Errorf("Render() =\n%s\nwant\n%s", got, want)
	}
}
