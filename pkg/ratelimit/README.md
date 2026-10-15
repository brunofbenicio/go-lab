# ratelimit

Token bucket rate limiter, safe for concurrent use, with an injectable clock
(`WithClock`) so tests never depend on real time.

## Usage

```go
l := ratelimit.New(10, 20) // 10 req/s, bursts of up to 20
if !l.Allow() {
    http.Error(w, "too many requests", http.StatusTooManyRequests)
    return
}
```

- `AllowN(n)` consumes n tokens at once, or none.
- `Reserve(n)` consumes the tokens and returns how long to wait before acting.
- `Tokens()` reports the available tokens.

## Tests

```sh
go test -race ./pkg/ratelimit/
```
