# `TestMain`: package-level setup and teardown

## What it solves

Some setup is expensive or global and should happen once per package, not once
per test: starting a service, creating a fixture directory, seeding a database,
parsing flags. `TestMain` runs before every test in the package and decides
the process exit code, which also makes it the place for the matching teardown.

## When to use

- Shared, read-only fixtures built once for the whole package.
- Global resources with a start/stop cost (containers, embedded servers).
- Custom flags that must be parsed before the tests start (`flag.Parse()`).

For anything that belongs to a single test, prefer `t.TempDir()` and
`t.Cleanup()` instead: they are scoped and cannot leak between tests.

## What this example shows

- `TestMain` calling `os.Exit(runTests(m))`.
- Why the work lives in `runTests`: `os.Exit` skips deferred calls, so the
  `defer os.RemoveAll(dir)` has to run inside a normal function.
- Fixture files written once and read by many tests through a package variable.
- A non-zero return when setup fails, so the package is reported as failed.

## Tips

- There is no `*testing.T` in `TestMain`, hence `os.MkdirTemp` plus manual removal.
- Only one `TestMain` is allowed per package.
- Do not mutate the shared fixtures from tests, or tests become order-dependent.

## Running

```sh
go test -v ./testing-examples/testmain/
```
