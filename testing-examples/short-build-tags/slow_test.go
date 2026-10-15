package primes

import "testing"

// Slow tests check themselves with testing.Short(). CI and local runs choose:
//
//	go test -short ./...   # skips this test: quick feedback while developing
//	go test ./...          # runs everything
func TestSieveLargeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the 5,000,000 limit in -short mode")
	}

	// pi(5,000,000) = 348,513 is a well-known value (OEIS A006880).
	got := Sieve(5_000_000)
	if len(got) != 348_513 {
		t.Fatalf("len(Sieve(5e6)) = %d, want 348513", len(got))
	}
	if last := got[len(got)-1]; last != 4_999_999 {
		t.Errorf("largest prime = %d, want 4999999", last)
	}
}
