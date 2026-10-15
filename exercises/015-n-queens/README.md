# 015 - N-Queens

## Problem

Place `n` queens on an `n x n` chessboard so that no two attack each other (no
shared row, column or diagonal). For small boards (`1 <= n <= 12`):

- `Count(n)` returns the number of solutions.
- `Solutions(n)` returns them all, where element `r` is the column of the queen
  in row `r`, in lexicographic order.
- `Render(solution)` draws one placement as text.

## Examples

```go
queens.Count(8)          // 92
sols, _ := queens.Solutions(4)
queens.Render(sols[0])
// .Q..
// ...Q
// Q...
// ..Q.
```

## Concepts

Backtracking with bitmasks for columns and diagonals, iterating over free
squares with `x & -x`, and cross-checking against a slow, obviously-correct
validator.

## Running the tests

```sh
go test -race ./exercises/015-n-queens/
```
