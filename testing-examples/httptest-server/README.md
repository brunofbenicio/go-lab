# HTTP clients with `httptest.NewServer`

## What it solves

An HTTP client has many moving parts: URL building, headers, status handling,
JSON decoding, timeouts and cancellation. `httptest.NewServer` starts a real
server on a random loopback port, so the client is tested end to end through
the actual `net/http` stack, without touching the internet.

## When to use

- Clients and SDKs wrapping an HTTP API.
- When behavior depends on real connections: TLS, cancellation, redirects, keep-alive.
- To assert exactly what the client *sent* (path, escaping, headers).

For handlers (the server side) prefer `httptest.NewRecorder`; for a client with
no network at all, see `fake-roundtripper`.

## What this example shows

- A `newClient` helper that starts the server and registers `srv.Close` with `t.Cleanup`.
- Request verification inside the handler, using `t.Errorf` (never `t.Fatal`
  from another goroutine).
- Status code mapping (404, 429 with `Retry-After`, 500, invalid JSON).
- Cancellation tested deterministically with channels instead of sleeps.
- `httptest.NewTLSServer`, `srv.Client()`, and `NewUnstartedServer` to tweak the server first.

## Tips

- `srv.Close` blocks until in-flight requests finish: make sure blocked
  handlers can return (note the cleanup order in the cancellation test).
- Use `srv.Client()` so the client trusts the test certificate and its
  transport is closed together with the server.
- Shared counters touched by handlers need `sync/atomic` or a mutex.

## Running

```sh
go test -race -v ./testing-examples/httptest-server/
```
