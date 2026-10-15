# ttlcache

Generic in-memory cache with per-entry time-to-live. Expiration is lazy
(checked on `Get`) and `Purge` removes everything that has already expired.
The clock is injectable (`WithClock`), so tests never sleep.

## Usage

```go
c := ttlcache.New[string, User](5 * time.Minute)
c.Set("42", user)                              // default TTL
c.SetWithTTL("43", other, 10*time.Second)      // custom TTL; <= 0 never expires
if u, ok := c.Get("42"); ok {
    use(u)
}
```

## Tests

```sh
go test -race ./pkg/ttlcache/
```
