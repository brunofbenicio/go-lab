# `testing.Short` and build tags

## What it solves

Not every test should run every time. Slow tests hurt the edit-compile-test
loop, and some tests need an environment (a database, extra services) that is
not always available. Go gives two switches: `-short`, checked at runtime with
`testing.Short()`, and **build tags**, which decide at compile time whether a
file takes part in the build.

## When to use

| Mechanism                | Use it for                                          |
| ------------------------ | --------------------------------------------------- |
| `testing.Short()` + Skip | tests that are only slow; on by default, off in `-short` |
| `//go:build integration` | tests that need a special environment; off by default |

## What this example shows

- `primes_test.go`: fast tests that always run.
- `slow_test.go`: `TestSieveLargeLimit` calls `t.Skip` when `testing.Short()` is true.
- `integration_test.go`: guarded by `//go:build integration`, it cross-checks the
  sieve against an independent oracle and only exists in builds using the tag.

## Running

```sh
go test ./testing-examples/short-build-tags/                      # fast + slow
go test -short -v ./testing-examples/short-build-tags/            # slow test is SKIPped
go test -tags integration ./testing-examples/short-build-tags/    # also the tagged file
go vet -tags integration ./testing-examples/short-build-tags/     # vet the tagged file too
```

## Tips

- Always give `t.Skip` a reason; it shows up in `-v` output.
- Remember to run `go vet` and your linters with the tags too, or the tagged
  files are never checked.
- Keep a single tag name per kind of environment (`integration`, `e2e`) and
  document it, as done here.
