# URL Shortener API

A small REST API that shortens URLs, redirects visitors to the original
destination and keeps simple usage counters. It is written with the Go standard
library only (`net/http` with the Go 1.22+ `ServeMux` patterns) and documented
with OpenAPI 3.0.3.

## Features

- Random, collision-safe codes: 7 base62 characters (`0-9A-Za-z`), drawn again
  automatically if a code is already taken.
- Strict URL validation: absolute `http`/`https` only, up to 2048 characters, no
  embedded credentials.
- Optional lifetime per link (`ttl_seconds`, up to one year). Expired links
  answer `410 Gone` but keep their statistics.
- Hit counter and last access time for every link.
- Thread-safe in-memory storage behind a small `store.Store` interface.
- Interactive documentation at `/docs` and the raw specification at
  `/openapi.yaml`.
- Graceful shutdown on `SIGINT`/`SIGTERM`.

## Running

```sh
go run ./apis/url-shortener/cmd/server
```

The server listens on port `8080` by default.

| Variable   | Default                  | Description                                                          |
|------------|--------------------------|----------------------------------------------------------------------|
| `PORT`     | `8080`                   | TCP port to listen on.                                               |
| `BASE_URL` | derived from the request | Public origin used to build `short_url`, e.g. `https://sho.example.com`. |

```sh
PORT=9000 BASE_URL=https://sho.example.com go run ./apis/url-shortener/cmd/server
```

Data lives in memory only: restarting the server forgets every link.

## Endpoints

| Method | Path                       | Description                          |
|--------|----------------------------|--------------------------------------|
| `POST` | `/api/links`               | Create a short link.                 |
| `GET`  | `/api/links/{code}/stats`  | Read the statistics of a link.       |
| `GET`  | `/{code}`                  | `302` redirect to the original URL.  |
| `GET`  | `/openapi.yaml`            | The OpenAPI document.                |
| `GET`  | `/docs`                    | Swagger UI (assets loaded from a CDN). |

### Create a link

```sh
curl -i -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://example.com/articles/2025/a-very-long-article-title"}'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json
Location: http://localhost:8080/api/links/ywZrEth/stats

{"code":"ywZrEth","short_url":"http://localhost:8080/ywZrEth","url":"https://example.com/articles/2025/a-very-long-article-title","created_at":"2025-03-01T12:00:00.224829197Z"}
```

With a lifetime of one hour:

```sh
curl -s -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://example.com/promo", "ttl_seconds": 3600}'
```

```json
{"code":"RsGczGi","short_url":"http://localhost:8080/RsGczGi","url":"https://example.com/promo","created_at":"2025-03-01T12:00:00.233413971Z","expires_at":"2025-03-01T13:00:00.233413971Z"}
```

### Follow a link

```sh
curl -i http://localhost:8080/ywZrEth
```

```http
HTTP/1.1 302 Found
Cache-Control: no-store
Location: https://example.com/articles/2025/a-very-long-article-title
```

Use `curl -L` to follow the redirect. After `ttl_seconds` the same request
answers `410 Gone` with `{"error":{"code":"link_expired", ...}}`.

### Read the statistics

```sh
curl -s http://localhost:8080/api/links/ywZrEth/stats
```

```json
{"code":"ywZrEth","url":"https://example.com/articles/2025/a-very-long-article-title","created_at":"2025-03-01T12:00:00.224829197Z","hits":1,"last_access_at":"2025-03-01T12:00:00.267475678Z","expired":false}
```

Reading the statistics does not count as a hit. `expires_at` and
`last_access_at` are omitted when they do not apply.

## Errors

Every error is a JSON object with a stable `code` and a human-readable
`message`:

```sh
curl -s -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' -d '{"url": "ftp://example.com"}'
```

```json
{"error":{"code":"invalid_url","message":"invalid URL: scheme must be http or https"}}
```

| Status | `error.code`             | When                                                       |
|--------|--------------------------|------------------------------------------------------------|
| 400    | `invalid_json`           | Malformed body, unknown field or wrong type.               |
| 404    | `not_found`              | Unknown or malformed code.                                 |
| 405    | _(plain text)_           | Wrong method for the path (`Allow` header is set).         |
| 410    | `link_expired`           | The link existed but its lifetime is over.                 |
| 413    | `payload_too_large`      | Request body above 8 KiB.                                  |
| 415    | `unsupported_media_type` | `Content-Type` is not `application/json`.                  |
| 422    | `invalid_url`            | The URL was rejected (scheme, host, credentials, length).  |
| 422    | `invalid_ttl`            | `ttl_seconds` is not between 1 and 31536000.               |
| 503    | `code_generation_failed` | Every candidate code collided; retrying is safe.           |
| 500    | `internal_error`         | Unexpected failure; details are only logged.               |

## Layout

```text
apis/url-shortener/
├── cmd/server/            wiring, configuration and graceful shutdown
├── internal/
│   ├── base62/            integer <-> base62 string encoding
│   ├── store/             Store interface and in-memory implementation
│   ├── shortener/         business rules: validation, codes, expiry, hits
│   ├── httpapi/           HTTP handlers and JSON errors
│   └── docs/              /openapi.yaml and the Swagger UI page
├── openapi.yaml           OpenAPI 3.0.3 specification
└── spec.go                embeds openapi.yaml into the binary
```

## Tests

```sh
go test -race -count=1 ./apis/url-shortener/...
go vet ./apis/url-shortener/...
```

The suite is deterministic: the clock and the random source are injected, no
test uses the real network (only `httptest`) and nothing sleeps. The end-to-end
test in `cmd/server/e2e_test.go` runs the whole flow over HTTP: creation, a code
collision, redirects, statistics, expiry and the error responses. Another test
checks that every registered route appears in `openapi.yaml`, and the other way
around.
