# 002 - Caesar cipher

## Problem

Implement the Caesar cipher:

- `Shift(s string, k int) string` moves every ASCII letter `k` positions,
  keeping case and leaving other characters alone. `k` can be negative or
  greater than 26.
- `Encrypt` and `Decrypt` are the friendly names for `Shift(s, k)` and
  `Shift(s, -k)`.
- `Crack(ciphertext string) (int, string)` recovers the key of an English text
  using letter-frequency analysis (chi-squared).

## Examples

```go
caesar.Encrypt("Hello, World!", 3) // "Khoor, Zruog!"
caesar.Decrypt("Khoor, Zruog!", 3) // "Hello, World!"
```

## Concepts

Modular arithmetic with negative numbers, `strings.Map`, and a simple
statistical attack.

## Running the tests

```sh
go test -race ./exercises/002-caesar-cipher/
```
