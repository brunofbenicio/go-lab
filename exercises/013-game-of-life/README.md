# 013 - Game of Life

## Problem

Implement Conway's Game of Life on a rectangular grid:

- A live cell with 2 or 3 live neighbors survives; otherwise it dies.
- A dead cell with exactly 3 live neighbors becomes alive.

The API:

- `New(rows, cols, wrap)` and `Parse(wrap, lines...)` (`#` alive, `.` dead).
- `Step()` returns the next state of the board without changing the original grid.
- `Alive`, `Set`, `Population`, `Equal` and `String` for inspection.

With `wrap` enabled the grid behaves like a torus; otherwise everything outside
the grid is dead.

## Examples

```
.....      .....
..#..      .....
..#..  ->  .###.   (blinker, period 2)
..#..      .....
.....      .....
```

## Concepts

Immutable board states, modular arithmetic for wrapping, and verifying patterns
(still life, oscillator, glider) rather than single cells.

## Running the tests

```sh
go test -race ./exercises/013-game-of-life/
```
