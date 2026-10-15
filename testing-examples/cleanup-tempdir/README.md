# `t.Cleanup` and `t.TempDir`

## What it solves

Tests that create files or open resources must leave nothing behind, even when
they fail halfway. `t.TempDir()` hands each test a private directory that is
deleted automatically, and `t.Cleanup(fn)` registers teardown code that runs
when the test ends, no matter how it ends.

## When to use

- Any test that touches the file system: never write to the working directory.
- Setup helpers that open a resource (file, server, connection) and must close it.
- Instead of `defer` when the cleanup belongs to a helper, not to the test body:
  a `defer` inside the helper would run when the helper returns, too early.

## What this example shows

- `newWriter`: a helper that creates a temp dir, opens the writer and registers its `Close`.
- File-based assertions: listing the directory and reading the rotated files.
- An idempotent `Close`, so an explicit close in the test and the cleanup do not clash.
- Cleanup order (last registered runs first) and automatic removal of the temp dir.

## Tips

- Each call to `t.TempDir()` returns a new, unique directory; subtests get their own.
- Register cleanups right after acquiring the resource, so a later `t.Fatal`
  cannot skip them.
- Use `t.Errorf` (not `t.Fatalf`) inside cleanup functions when possible.

## Running

```sh
go test -v ./testing-examples/cleanup-tempdir/
```
