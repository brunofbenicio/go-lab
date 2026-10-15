//go:build integration

package primes

import "testing"

// This file is only compiled when the "integration" build tag is given:
//
//	go test -tags integration ./testing-examples/short-build-tags/
//
// Without the tag the file does not exist as far as the compiler is concerned,
// so slow or environment-dependent tests never run by accident. The
// //go:build line must come first and be followed by a blank line.

// TestSieveAgreesWithTrialDivision cross-checks the fast implementation
// against the slow oracle for every number up to the limit.
func TestSieveAgreesWithTrialDivision(t *testing.T) {
	const limit = 100_000

	isPrime := make([]bool, limit+1)
	for _, p := range Sieve(limit) {
		isPrime[p] = true
	}
	for n := 0; n <= limit; n++ {
		if isPrime[n] != IsPrime(n) {
			t.Fatalf("n=%d: sieve says %v, trial division says %v", n, isPrime[n], IsPrime(n))
		}
	}
}
