# circuitbreaker

Circuit breaker: after too many consecutive failures the circuit opens and
calls are rejected immediately with `ErrOpen`, sparing a struggling dependency.
After a cooldown, one trial call decides whether the circuit closes again.

## Usage

```go
b := circuitbreaker.New(
    circuitbreaker.WithThreshold(5),
    circuitbreaker.WithCooldown(30*time.Second),
)
err := b.Execute(func() error { return callPayments(ctx) })
if errors.Is(err, circuitbreaker.ErrOpen) {
    // degrade gracefully
}
```

States are `Closed`, `Open` and `HalfOpen`. `OnStateChange` reports each
transition, and the injectable clock keeps tests free of real waiting.

## Tests

```sh
go test -race ./pkg/circuitbreaker/
```
