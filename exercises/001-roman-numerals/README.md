# 001 - Roman numerals

## Problem

Write two functions:

- `ToRoman(n int) (string, error)` converts an integer in `1..3999` to a Roman
  numeral.
- `FromRoman(s string) (int, error)` parses an upper-case Roman numeral.

Only the canonical spelling is valid: `IV` is accepted, `IIII` is not. Values
outside the supported range return `ErrOutOfRange`; malformed numerals return
`ErrInvalid`.

## Examples

| Input       | Output      |
|-------------|-------------|
| `4`         | `IV`        |
| `1994`      | `MCMXCIV`   |
| `"MMXXIV"`  | `2024`      |
| `"IIII"`    | `ErrInvalid` |

## Concepts

Greedy algorithm over a descending table, subtractive pairs, and validating
input by converting it back.

## Running the tests

```sh
go test -race ./exercises/001-roman-numerals/
```
