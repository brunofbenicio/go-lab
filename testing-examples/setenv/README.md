# Environment variables with `t.Setenv`

## What it solves

Code that reads `os.Getenv` depends on the machine running the test. Calling
`os.Setenv` in a test pollutes every test that runs afterwards. `t.Setenv`
sets the variable for the current test only and restores the previous state
(including "was not set") when the test ends.

## When to use

- Configuration loaders, feature flags, anything reading `APP_*` or `HOME`.
- Making a test independent of the developer's or CI's environment.

## What this example shows

- Defaults, overrides and invalid values for a config loaded from the environment.
- `cleanEnv`/`unsetenv`: forcing a variable to be *absent* (`t.Setenv` followed by
  `os.Unsetenv`), which matters because `os.LookupEnv` distinguishes unset from empty.
- Restoration after a subtest finishes.

## Caveats

- `t.Setenv` panics in a test that called `t.Parallel()` (or whose parent did):
  the environment is process-wide, so such tests cannot overlap. The same
  applies to `t.Chdir`.
- If the code can take its values from a parameter (a `lookup func(string) string`),
  injecting it is often cleaner than touching the real environment.

## Running

```sh
go test -v ./testing-examples/setenv/
APP_PORT=1234 go test -v ./testing-examples/setenv/   # outer value does not matter
```
