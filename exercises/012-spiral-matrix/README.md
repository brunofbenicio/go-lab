# 012 - Spiral matrix

## Problem

- `Spiral(m [][]int) ([]int, error)` reads a rectangular matrix in clockwise
  spiral order, starting at the top-left corner. Rows of different lengths
  return `ErrRagged`.
- `Fill(rows, cols int) [][]int` does the reverse: it writes `1..rows*cols` into
  a new matrix following the same spiral.

## Examples

```
1 2 3        Spiral -> [1 2 3 6 9 8 7 4 5]
4 5 6
7 8 9
```

```go
spiral.Fill(3, 3) // [[1 2 3] [8 9 4] [7 6 5]]
```

## Concepts

Shrinking boundaries versus direction vectors, edge cases with a single row or
column, and testing two functions as inverses of each other.

## Running the tests

```sh
go test -race ./exercises/012-spiral-matrix/
```
