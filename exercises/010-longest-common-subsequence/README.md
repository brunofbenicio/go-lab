# 010 - Longest common subsequence

## Problem

A subsequence keeps the relative order of characters but may skip some. Given
two strings, find the longest subsequence they have in common.

- `Length(a, b string) int` returns its length.
- `LCS(a, b string) string` returns one such subsequence.

Work with runes so that accented characters count as one.

## Examples

```go
lcs.LCS("AGGTAB", "GXTXAYB")   // "GTAB"
lcs.Length("ABCBDAB", "BDCABA") // 4
```

## Concepts

Bottom-up dynamic programming, reconstructing an answer by walking the table,
and properties that hold for any valid answer.

## Running the tests

```sh
go test -race ./exercises/010-longest-common-subsequence/
```
