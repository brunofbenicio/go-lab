// Package sieve implements the Sieve of Eratosthenes.
package sieve

import "math"

// Primes returns every prime number less than or equal to limit, in
// ascending order. It returns nil when limit < 2.
func Primes(limit int) []int {
	if limit < 2 {
		return nil
	}
	composite := make([]bool, limit+1)
	// Only multiples starting at i*i need marking: smaller ones were already
	// crossed out by a smaller prime factor.
	for i := 2; i*i <= limit; i++ {
		if composite[i] {
			continue
		}
		for j := i * i; j <= limit; j += i {
			composite[j] = true
		}
	}
	var primes []int
	for i := 2; i <= limit; i++ {
		if !composite[i] {
			primes = append(primes, i)
		}
	}
	return primes
}

// Count returns how many primes are less than or equal to limit.
func Count(limit int) int {
	return len(Primes(limit))
}

// Nth returns the n-th prime (Nth(1) == 2). The boolean is false when n < 1.
func Nth(n int) (int, bool) {
	if n < 1 {
		return 0, false
	}
	// For n >= 6 the n-th prime is below n*(ln n + ln ln n); use a fixed
	// bound for the first few primes.
	limit := 15
	if n >= 6 {
		x := float64(n)
		limit = int(x*(math.Log(x)+math.Log(math.Log(x)))) + 1
	}
	primes := Primes(limit)
	return primes[n-1], true
}
