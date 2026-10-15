// Package primes finds prime numbers. The sieve is fast enough to be unit
// tested, but large inputs make for slow tests, which is the occasion to use
// testing.Short and build tags to split a suite into fast and slow parts.
package primes

// IsPrime reports whether n is prime, by trial division. It is slow for big
// numbers but obviously correct, which makes it a good reference oracle.
func IsPrime(n int) bool {
	if n < 2 {
		return false
	}
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}

// Sieve returns every prime less than or equal to limit, in ascending order,
// using the sieve of Eratosthenes.
func Sieve(limit int) []int {
	if limit < 2 {
		return nil
	}
	composite := make([]bool, limit+1)
	var primes []int
	for i := 2; i <= limit; i++ {
		if composite[i] {
			continue
		}
		primes = append(primes, i)
		for j := i * i; j <= limit; j += i {
			composite[j] = true
		}
	}
	return primes
}
