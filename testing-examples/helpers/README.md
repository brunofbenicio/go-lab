# Test helpers with `t.Helper`

## What it solves

Repeated assertions get extracted into small functions. Without `t.Helper()`,
a failure is reported at the line *inside* the helper, which says nothing
about which test case broke. Marking the function as a helper makes Go report
the caller's line instead.

## When to use

- Whenever the same check (`got != want` plus a message) appears in several tests.
- For setup that must abort the test on failure (`Fatalf`), such as loading a fixture.
- For helpers shared by tests and benchmarks: accept `testing.TB`.

## What this example shows

- `assertEqual[T comparable]`, `assertLines`, `assertErrorIs` and `mustWrap`,
  all starting with `t.Helper()`.
- The `Errorf` (continue) versus `Fatalf` (stop) distinction in helpers.
- How to test the helpers themselves: a `spyTB` struct embeds `testing.TB`,
  overrides `Helper`, `Errorf` and `Fatalf`, and runs the helper in its own
  goroutine so `runtime.Goexit` (what `Fatalf` does) is contained.

## Tips

- Put `t.Helper()` first, before any other statement.
- Keep helpers dumb: they should not hide the scenario being tested.
- Helpers that need cleanup can call `t.Cleanup` themselves.

## Running

```sh
go test -v ./testing-examples/helpers/
```

To see the effect, change an expected value in `TestWrap` and compare the
reported line with and without `t.Helper()` in `assertLines`.
