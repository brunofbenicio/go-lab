# 014 - Sudoku validator

## Problem

Validate Sudoku boards:

- `Parse(s string) (Board, error)` reads 81 cells (digits `1-9`, with `0` or `.`
  for empty), ignoring whitespace so the board can span nine lines.
- `Board.Validate() error` checks that no digit repeats in any row, column or
  3x3 box. Empty cells are ignored, so partial boards work. A repeated digit
  returns a `*ConflictError` naming the unit, its 1-based index and the digit.
- `Board.Solved() bool` is true for a full, valid board.

## Examples

```go
b, _ := sudoku.Parse("53..7.... 6..195... .98....6. ...") // 9 groups of 9
err := b.Validate() // digit 5 repeated in row 1
```

## Concepts

Bitmasks for "seen" sets, expressing rows, columns and boxes with one loop, and
returning structured errors that callers can inspect with `errors.As`.

## Running the tests

```sh
go test -race ./exercises/014-sudoku-validator/
```
