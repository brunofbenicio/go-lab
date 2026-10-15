# 007 - Valid brackets

## Problem

Check whether the brackets `()`, `[]` and `{}` in a string are balanced:
every opener has a closer of the same kind, closed in the right order.

- `Valid(s string) bool` answers yes or no.
- `Check(s string) error` returns a `*SyntaxError` with the byte position of the
  first problem (an unexpected closer, a mismatch, or an opener never closed).

Any character that is not a bracket is ignored.

## Examples

```go
brackets.Valid("{[()]}")  // true
brackets.Valid("([)]")    // false
brackets.Check("a(b]c")   // brackets: position 3: found ']' but expected ')'
```

## Concepts

Stack-based parsing, custom error types with `errors.As`, and `range` over
strings (byte offsets).

## Running the tests

```sh
go test -race ./exercises/007-valid-brackets/
```
