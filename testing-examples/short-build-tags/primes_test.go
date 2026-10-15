package primes

import (
	"slices"
	"testing"
)

// Fast tests: always run, including under `go test -short`.

func TestSieveSmall(t *testing.T) {
	tests := []struct {
		limit int
		want  []int
	}{
		{-5, nil},
		{1, nil},
		{2, []int{2}},
		{10, []int{2, 3, 5, 7}},
		{30, []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29}},
	}
	for _, tt := range tests {
		if got := Sieve(tt.limit); !slices.Equal(got, tt.want) {
			t.Errorf("Sieve(%d) = %v, want %v", tt.limit, got, tt.want)
		}
	}
}

func TestIsPrime(t *testing.T) {
	primes := map[int]bool{-7: false, 0: false, 1: false, 2: true, 3: true, 4: false, 97: true, 100: false, 7919: true}
	for n, want := range primes {
		if got := IsPrime(n); got != want {
			t.Errorf("IsPrime(%d) = %v, want %v", n, got, want)
		}
	}
}
