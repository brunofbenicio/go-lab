# Parallel tests with `t.Parallel`

## What it solves

Tests normally run one after another. Independent, CPU- or wait-bound tests can
overlap with `t.Parallel()`, which shortens the suite and, together with
`-race`, quickly exposes accidental shared state between tests.

## When to use

- Tests with no shared mutable state (pure functions, a private fixture per test).
- Suites with many slow-ish subtests (I/O, loopback servers).
- As a cheap smoke test for hidden globals: if it breaks in parallel, it was coupled.

## What this example shows

- `t.Parallel()` in a top-level test and in each table-driven subtest.
- Since Go 1.22 the loop variable is per-iteration, so `tt := tt` is unnecessary.
- A parent `t.Cleanup` that only runs after all parallel subtests finish.
- The "group" pattern: a serial `t.Run` that returns once its parallel children are done.

## Caveats

- Tests that call `t.Setenv` or `t.Chdir` cannot be parallel.
- Parallel subtests share the parent's data: read-only is fine, writes need a lock.
- The number of simultaneous tests is limited by `-parallel N`.

## Running

```sh
go test -race ./testing-examples/parallel/
go test -v -parallel 2 ./testing-examples/parallel/
```
