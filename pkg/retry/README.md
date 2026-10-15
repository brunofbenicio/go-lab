# retry

Repeats an operation until it succeeds, attempts run out or the context is
cancelled, using the `backoff` package to compute the wait between attempts.

## Usage

```go
err := retry.Do(ctx, func(ctx context.Context) error {
    return callService(ctx)
}, retry.Attempts(5), retry.RetryIf(isTemporary))
```

- `retry.Permanent(err)` stops retrying immediately.
- `DoValue` also returns the operation's result.
- `ErrExhausted` wraps the last error; inspect it with `errors.Is`.
- `WithSleep` swaps the wait for a fake function in tests.

## Tests

```sh
go test -race ./pkg/retry/
```
