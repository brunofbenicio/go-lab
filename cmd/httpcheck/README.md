# httpcheck

Requests one or more URLs and reports whether each one answered with an
acceptable status. Handy for smoke tests and health checks in scripts.

## Usage

```sh
httpcheck [flags] url ...
```

| Flag           | Meaning                                                   |
|----------------|-----------------------------------------------------------|
| `-method M`    | HTTP method (default `GET`)                               |
| `-expect N`    | exact status code expected (default: any status below 400) |
| `-timeout D`   | time limit per request, e.g. `500ms` (default `5s`)       |
| `-no-redirect` | report redirects (3xx) instead of following them          |
| `-c N`         | requests in flight at once (default `4`)                  |

URLs must start with `http://` or `https://`. Requests run concurrently, but
results are printed in the order the URLs were given:

```
OK   200 https://example.com/
FAIL 404 https://example.com/missing
FAIL ERR https://example.com:81/ (timeout after 5s)
```

Exit status: `0` when every URL is acceptable, `1` when at least one is not,
`2` for usage errors.

## Examples

```sh
httpcheck https://example.com/ https://example.com/health
httpcheck -expect 401 https://example.com/admin      # must be protected
httpcheck -method HEAD -timeout 2s https://example.com/
httpcheck -no-redirect http://example.com/old        # shows the 301/302 itself
```

## Tests

The tests start local `httptest` servers on the loopback interface; they never
touch the network.

```sh
go test -race ./cmd/httpcheck/
```
