package sieve

import (
	"reflect"
	"testing"
)

func TestPrimes(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  []int
	}{
		{"negative", -10, nil},
		{"zero", 0, nil},
		{"one", 1, nil},
		{"two", 2, []int{2}},
		{"ten", 10, []int{2, 3, 5, 7}},
		{"limit is prime", 13, []int{2, 3, 5, 7, 11, 13}},
		{"thirty", 30, []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Primes(tt.limit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Primes(%d) = %v, want %v", tt.limit, got, tt.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		limit, want int
	}{
		{1, 0},
		{100, 25},
		{1000, 168},
		{10000, 1229},
		{100000, 9592},
	}
	for _, tt := range tests {
		if got := Count(tt.limit); got != tt.want {
			t.Errorf("Count(%d) = %d, want %d", tt.limit, got, tt.want)
		}
	}
}

func TestNth(t *testing.T) {
	tests := []struct {
		n      int
		want   int
		wantOK bool
	}{
		{-1, 0, false},
		{0, 0, false},
		{1, 2, true},
		{5, 11, true},
		{6, 13, true},
		{10, 29, true},
		{1000, 7919, true},
		{10000, 104729, true},
	}
	for _, tt := range tests {
		got, ok := Nth(tt.n)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("Nth(%d) = (%d, %v), want (%d, %v)", tt.n, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestNthMatchesPrimes(t *testing.T) {
	primes := Primes(5000)
	for i, p := range primes {
		if got, ok := Nth(i + 1); !ok || got != p {
			t.Fatalf("Nth(%d) = (%d, %v), want %d", i+1, got, ok, p)
		}
	}
}
