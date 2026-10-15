# 004 - Prime sieve

## Problem

Implement the Sieve of Eratosthenes:

- `Primes(limit int) []int` returns all primes up to and including `limit`.
- `Count(limit int) int` returns how many there are.
- `Nth(n int) (int, bool)` returns the n-th prime (`Nth(1) == 2`) by sizing the
  sieve with the bound `n * (ln n + ln ln n)`.

## Examples

```go
sieve.Primes(30)   // [2 3 5 7 11 13 17 19 23 29]
sieve.Count(100)   // 25
sieve.Nth(1000)    // 7919
```

## Concepts

Marking multiples starting at `i*i`, stopping at the square root of the limit,
and using a mathematical bound to size a buffer.

## Running the tests

```sh
go test -race ./exercises/004-prime-sieve/
```
