# slidingwindow

Per-key rate limiter with an exact sliding window: it stores the instant of
every accepted request and never allows more than `limit` requests in any
interval of length `window`.

## Usage

```go
l := slidingwindow.New(100, time.Minute) // 100 req/min per key
if !l.Allow(userID) {
    w.Header().Set("Retry-After", strconv.Itoa(int(l.RetryAfter(userID).Seconds())))
}
```

- `Remaining(key)` reports the remaining quota.
- `RetryAfter(key)` reports how long until the next request is accepted.
- `Cleanup()` removes inactive keys; the clock is injectable via `WithClock`.

## Tests

```sh
go test -race ./pkg/slidingwindow/
```
