# HTTP handlers with `httptest.NewRecorder`

## What it solves

An `http.Handler` is just a function from a request to a response writer, so it
can be tested by calling it directly. `httptest.NewRequest` builds the request
and `httptest.NewRecorder` captures what the handler wrote. No port is opened
and no network is involved, so these tests are fast and deterministic.

## When to use

- Handlers, routers and middleware: status codes, headers, JSON bodies.
- Validation and error paths that are tedious to reach through a real server.
- When you do *not* need real connections, TLS or a client; for those, see
  `httptest-server`.

## What this example shows

- A `do` helper wrapping request creation, `ServeHTTP` and `rec.Result()`.
- Status, headers (`Location`, `Content-Type`) and decoded JSON body assertions.
- A table of error responses, including the router's own 404 and 405.
- `rec.Code` and `rec.Body` as shortcuts, versus `rec.Result()` for a real `*http.Response`.
- Calling one handler method directly, setting the wildcard with `req.SetPathValue`.

## Tips

- `httptest.NewRequest` panics on a malformed target, which suits tests;
  `http.NewRequest` returns an error instead.
- Test the router (`Handler()`), not only individual functions: it covers the
  method and path matching too.

## Running

```sh
go test -race -v ./testing-examples/httptest-recorder/
```
