# semaphore

Counting semaphore that limits how many goroutines run a section at the same
time. Acquisition respects `context.Context`, and `Do` guarantees the slot is
released even when the function panics.

## Usage

```go
sem := semaphore.New(5) // at most 5 downloads at once
err := sem.Do(ctx, func() error {
    return download(ctx, url)
})
```

- `TryAcquire` never blocks; `Acquire`/`Release` give manual control.
- `Release` without a matching `Acquire` panics, exposing the bug early.

## Tests

```sh
go test -race ./pkg/semaphore/
```
