# 019 - Duration parser

## Problem

`time.ParseDuration` does not understand days or weeks. Write your own:

- `Parse(s string) (time.Duration, error)` accepts a sequence of
  `<number><unit>` pairs with an optional sign: `1w2d3h`, `90m`, `-45s`, `1.5h`.
  Units: `ns`, `us`/`µs`, `ms`, `s`, `m`, `h`, `d`, `w`.
- `Format(d time.Duration) string` is the inverse, e.g. `1d2h30m`, so that
  `Parse(Format(d)) == d`.

Return distinct errors for bad syntax (`ErrSyntax`), unknown units
(`ErrUnknownUnit`) and values that do not fit in a `time.Duration`
(`ErrOverflow`). Overflow must be detected, never silently wrapped.

## Examples

```go
duration.Parse("1d2h30m") // 26h30m0s
duration.Parse("1.5h")    // 1h30m0s
duration.Format(8 * duration.Day) // "1w1d"
```

## Concepts

Hand-written parsing, overflow-safe arithmetic with `math/bits`, and
round-trip tests.

## Running the tests

```sh
go test -race ./exercises/019-duration-parser/
```
