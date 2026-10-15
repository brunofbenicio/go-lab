# retryhttp

An `http.RoundTripper` that retries failed requests with backoff, built on the
`backoff` package. Plug it into any `http.Client`.

## Usage

```go
client := &http.Client{
    Transport: retryhttp.New(nil, retryhttp.Attempts(4)),
    Timeout:   10 * time.Second,
}
resp, err := client.Get("https://example.com/api")
```

- Retries network errors and the statuses 429, 502, 503 and 504.
- Honors a numeric `Retry-After` header, capped by `WithMaxRetryAfter`.
- Only repeats requests that are safe to repeat: idempotent methods (or an
  `Idempotency-Key` header) whose body can be replayed via `GetBody`.
- Drains and closes discarded responses so connections are reused.
- `WithSleep` makes tests instantaneous.

## Tests

```sh
go test -race ./pkg/retryhttp/
```
