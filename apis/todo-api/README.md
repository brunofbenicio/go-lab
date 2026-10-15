# todo-api

A small REST API for managing a todo list, written with the Go standard
library only. It is meant as a reference for how a production-minded HTTP
service can be structured without a framework: strict input handling,
middleware, graceful shutdown, an OpenAPI document with Swagger UI, and tests
at every level.

- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Endpoints](#endpoints)
- [Examples](#examples)
- [Documentation (OpenAPI and Swagger UI)](#documentation-openapi-and-swagger-ui)
- [Project layout](#project-layout)
- [Design notes](#design-notes)
- [Development](#development)

## Quick start

```sh
# from the repository root
go run ./apis/todo-api/cmd/server
# or, from this directory
make run
```

The server listens on `:8080`. Open <http://localhost:8080/docs> for the
interactive documentation.

Run it with an API key to protect the write operations:

```sh
TODO_API_KEYS=change-me go run ./apis/todo-api/cmd/server
```

Data lives in memory, so it disappears when the process stops. The store is
behind the `todo.Store` interface, which is where a database would plug in.

## Configuration

Everything is configured through environment variables. Invalid values stop the
server at startup with a clear message.

| Variable | Default | Meaning |
|----------|---------|---------|
| `PORT` | `8080` | TCP port, 1-65535 |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `TODO_API_KEYS` | *(none)* | Comma-separated API keys. When unset, writes are open (a warning is logged) |
| `CORS_ORIGINS` | `*` | Comma-separated origins allowed to call the API from a browser |
| `RATE_LIMIT_RPS` | `20` | Sustained requests per second allowed per client IP |
| `RATE_LIMIT_BURST` | `40` | Requests a client may send at once |
| `SHUTDOWN_TIMEOUT` | `10s` | How long in-flight requests get to finish after SIGINT/SIGTERM |

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/healthz` | - | Liveness probe |
| `GET` | `/api/todos` | - | List todos, with filters and paging |
| `POST` | `/api/todos` | key | Create a todo |
| `GET` | `/api/todos/{id}` | - | Get one todo |
| `PUT` | `/api/todos/{id}` | key | Replace a todo (omitted fields are reset) |
| `PATCH` | `/api/todos/{id}` | key | Update only the fields you send |
| `DELETE` | `/api/todos/{id}` | key | Delete a todo |
| `GET` | `/openapi.yaml` | - | The OpenAPI 3 document |
| `GET` | `/docs` | - | Swagger UI |

"key" means an API key is required when the server was started with
`TODO_API_KEYS`. Send it as `X-API-Key: <key>` or `Authorization: Bearer <key>`.

List query parameters: `done=true|false`, `q=<text>` (case-insensitive search in
title and notes), `limit=1..200` (default 50) and `offset>=0`. The response
carries the number of matches in the body (`total`) and in the `X-Total-Count`
header.

Every error uses one envelope:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "the request body is invalid",
    "details": [{ "field": "title", "message": "must not be empty" }]
  }
}
```

| Status | `code` | When |
|--------|--------|------|
| 400 | `invalid_json`, `invalid_id`, `invalid_query` | Malformed body, unknown field, bad id or query parameter |
| 401 | `unauthorized` | Missing or wrong API key on a write |
| 404 | `not_found` | The todo does not exist |
| 413 | `payload_too_large` | Body above 1 MiB |
| 415 | `unsupported_media_type` | Body is not `application/json` |
| 422 | `validation_failed` | Business rules broken, with one detail per field |
| 429 | `rate_limited` | Too many requests, see `Retry-After` |
| 500 | `internal_error` | Unexpected failure, details only in the server log |

## Examples

Create a todo:

```sh
curl -s -X POST localhost:8080/api/todos \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: change-me' \
  -d '{"title":"Ship the API","notes":"with swagger","due_date":"2026-12-31T10:00:00Z"}'
```

```json
{"id":1,"title":"Ship the API","notes":"with swagger","done":false,"due_date":"2026-12-31T10:00:00Z","created_at":"2026-10-15T14:26:55.887085108Z","updated_at":"2026-10-15T14:26:55.887085108Z"}
```

Mark it as done (only the sent fields change; `"due_date": null` would clear
the due date):

```sh
curl -s -X PATCH localhost:8080/api/todos/1 \
  -H 'Content-Type: application/json' -H 'X-API-Key: change-me' \
  -d '{"done":true}'
```

List the finished ones, two per page:

```sh
curl -s -i 'localhost:8080/api/todos?done=true&limit=2&offset=0'
```

```
HTTP/1.1 200 OK
X-Request-Id: f584b21b3000fda1d08f06704c2fa339
X-Total-Count: 1

{"items":[{"id":1,"title":"Ship the API", ...}],"total":1,"limit":2,"offset":0}
```

A write without the key is rejected:

```sh
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/api/todos \
  -H 'Content-Type: application/json' -d '{"title":"x"}'
# 401
```

## Documentation (OpenAPI and Swagger UI)

[`openapi.yaml`](openapi.yaml) describes every endpoint, schema, header and error.
It is embedded in the binary, so a running server always serves the version it
was built with:

- <http://localhost:8080/openapi.yaml> - the raw document, with an `ETag`
- <http://localhost:8080/docs> - Swagger UI. It loads `swagger-ui-dist` from the
  jsDelivr CDN, pinned to one version and protected with Subresource Integrity
  hashes, under a strict Content-Security-Policy. Use "Authorize" to enter an
  API key and try the write operations.

You can also import the document into Postman, Insomnia or a code generator
such as `openapi-generator`.

A test keeps the document honest: it fails when the router serves an endpoint
that the spec does not describe, or the other way around.

## Project layout

```
apis/todo-api/
├── cmd/server/          entrypoint: config, logger, signals
├── openapi.yaml         API contract (embedded by spec.go)
├── spec.go              go:embed of the contract
└── internal/
    ├── app/             assembles router + middleware from the config
    ├── config/          environment variables -> Config
    ├── docs/            /openapi.yaml and the Swagger UI page
    ├── httpapi/         routes, handlers, JSON decoding and error envelope
    ├── middleware/      request id, logging, recover, CORS, rate limit, API key
    ├── server/          graceful shutdown around http.Server
    └── todo/            domain model, validation and the Store interface
```

## Design notes

**Middleware order matters.** From the outside in: request id, logging, panic
recovery, CORS, rate limiting, API key. Logging wraps recovery so it sees the 500
that a panic becomes; CORS answers preflights before they cost rate-limit tokens
or need credentials; rate limiting runs before the key check so guessing keys is
throttled. The reasoning is documented next to `app.NewHandler`.

**Strict decoding.** Request bodies are capped at 1 MiB, unknown fields and
trailing data are rejected, and each failure mode maps to its own status and
error code.

**PATCH semantics.** `OptionalTime` tells an omitted `due_date` apart from an
explicit `null`, which a plain pointer cannot do. The store validates the merged
result, so a patch can never leave a todo in an invalid state.

**Safe comparisons.** API keys are hashed and compared in constant time without
early exit, so timing reveals neither the key nor its length.

**Testable by construction.** Clocks are injected (store, rate limiter), the
router and middleware are plain `http.Handler`s, and `server.Run` takes a
listener and a context, so every layer is tested without sleeping or opening
fixed ports.

## Development

```sh
make help        # list the targets
make check       # gofmt check, go vet and the race-enabled tests
make cover       # coverage summary
make bench       # benchmarks
make build       # static binary in bin/
make docker-build
```

The same commands without `make`, from the repository root:

```sh
go test -race ./apis/todo-api/...
go vet ./apis/todo-api/...
```

### Container

```sh
docker build -f apis/todo-api/Dockerfile -t todo-api .
docker run --rm -p 8080:8080 -e TODO_API_KEYS=change-me todo-api
```

The image is a static binary on `distroless/static`, running as a non-root user.
