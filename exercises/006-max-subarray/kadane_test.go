package kadane

import (
	"math/rand"
	"testing"
)

func TestMaxSubarray(t *testing.T) {
	tests := []struct {
		name   string
		in     []int
		want   Result
		wantOK bool
	}{
		{"empty", nil, Result{}, false},
		{"single", []int{5}, Result{5, 0, 0}, true},
		{"classic", []int{-2, 1, -3, 4, -1, 2, 1, -5, 4}, Result{6, 3, 6}, true},
		{"all positive", []int{1, 2, 3}, Result{6, 0, 2}, true},
		{"all negative", []int{-8, -3, -6, -2, -5}, Result{-2, 3, 3}, true},
		{"restarts after negative prefix", []int{-5, 4, 5}, Result{9, 1, 2}, true},
		{"with zeros", []int{0, 0, 0}, Result{0, 0, 0}, true},
		{"two disjoint peaks", []int{5, -10, 6}, Result{6, 2, 2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := MaxSubarray(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("MaxSubarray(%v) = (%+v, %v), want (%+v, %v)",
					tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// bruteForce returns the best sum by checking every subarray.
func bruteForce(xs []int) int {
	best := xs[0]
	for i := range xs {
		sum := 0
		for j := i; j < len(xs); j++ {
			sum += xs[j]
			best = max(best, sum)
		}
	}
	return best
}

func TestMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 300; round++ {
		xs := make([]int, 1+rng.Intn(25))
		for i := range xs {
			xs[i] = rng.Intn(21) - 10
		}
		got, ok := MaxSubarray(xs)
		if !ok {
			t.Fatalf("MaxSubarray(%v) reported empty", xs)
		}
		if want := bruteForce(xs); got.Sum != want {
			t.Fatalf("MaxSubarray(%v).Sum = %d, want %d", xs, got.Sum, want)
		}
		// The reported range must actually add up to the reported sum.
		sum := 0
		for _, v := range xs[got.Start : got.End+1] {
			sum += v
		}
		if sum != got.Sum {
			t.Fatalf("MaxSubarray(%v): range sums to %d, reported %d", xs, sum, got.Sum)
		}
	}
}
