# backoff

Wait strategies between retries: constant, linear and exponential, with an
optional ceiling and jitter variants (`FullJitter` and `EqualJitter`) so that
many clients do not hit a server again at the same moment.

## Usage

```go
s := backoff.EqualJitter(backoff.Exponential(100*time.Millisecond, 2, 5*time.Second), nil)
wait := s(attempt) // attempt starts at 0
```

The random source is injectable (`Rand`), which keeps tests deterministic.

## Tests

```sh
go test -race ./pkg/backoff/
```
