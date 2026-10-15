# Webhook Receiver API

A webhook endpoint that is safe by default. It authenticates every delivery with
an HMAC signature, refuses stale (replayed) requests, detects deliveries that a
sender retries and bounds the size of what it reads. It uses only the Go
standard library (`crypto/hmac`, `net/http` with the Go 1.22+ `ServeMux`
patterns) and is documented with OpenAPI 3.0.3.

## Features

- **HMAC-SHA-256 signatures** in the `sha256=<hex>` form, compared in constant
  time. The signed text is `<id>.<timestamp>.<body>`, so a captured request can
  neither be replayed with a fresh timestamp nor with a new delivery ID.
- **Replay protection**: the timestamp must be within a tolerance of the server
  clock (5 minutes by default), in both directions.
- **Idempotency**: deliveries are deduplicated by their ID for a configurable
  time. A retry of an already received delivery gets `409` and is not processed
  twice. If processing fails, the ID is released so the sender's retry is
  processed.
- **Body size limit** (64 KiB by default), enforced before and while reading.
- **No oracle for attackers**: every signature failure gets the same answer, and
  requests that fail authentication never consume a delivery ID.
- Interactive documentation at `/docs` and the raw specification at
  `/openapi.yaml`.
- Graceful shutdown on `SIGINT`/`SIGTERM`.

## Running

The shared secret is read from the environment and the server refuses to start
without it. Use any random string of at least 16 bytes:

```sh
export WEBHOOK_SECRET="$(openssl rand -hex 32)"
go run ./apis/webhook-receiver/cmd/server
```

| Variable              | Default    | Description                                                              |
|-----------------------|------------|--------------------------------------------------------------------------|
| `WEBHOOK_SECRET`      | _required_ | Shared HMAC secret, at least 16 bytes. Never logged.                     |
| `PORT`                | `8080`     | TCP port to listen on.                                                   |
| `MAX_BODY_BYTES`      | `65536`    | Largest accepted request body, up to 10 MiB.                             |
| `TIMESTAMP_TOLERANCE` | `5m`       | How far a delivery's timestamp may be from the server clock (up to `1h`). |
| `IDEMPOTENCY_TTL`     | `24h`      | How long delivery IDs are remembered (up to `720h`).                     |

`IDEMPOTENCY_TTL` must be at least twice `TIMESTAMP_TOLERANCE`; the server
refuses to start otherwise. The reason: a request accepted now can still pass the
timestamp check for up to twice the tolerance, and once its delivery ID is
forgotten nothing else would stop it from running again.

Accepted events are logged (delivery ID, type and payload size; never the
payload). Delivery IDs live in memory, so restarting the server forgets them.

## The protocol

Every request to `POST /webhooks` carries three headers:

| Header                | Value                                                                 |
|-----------------------|-----------------------------------------------------------------------|
| `X-Webhook-Id`        | A unique ID for the delivery. A retry of the same delivery reuses it. |
| `X-Webhook-Timestamp` | The time of the attempt, in Unix seconds (not milliseconds).          |
| `X-Webhook-Signature` | `sha256=` followed by the hex HMAC-SHA-256 of `<id>.<timestamp>.<body>`. |

The body is a JSON object with a `type` (lowercase letters, digits, `.`, `_`
and `-`, such as `order.created`) and an optional `data` of any JSON type.
Unknown top-level fields are ignored.

### Sending a delivery

```sh
BODY='{"type":"order.created","data":{"id":42}}'
ID=evt_001
TS=$(date +%s)
SIG=$(printf '%s.%s.%s' "$ID" "$TS" "$BODY" \
  | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -hex | sed 's/^.* //')

curl -i -X POST http://localhost:8080/webhooks \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Id: $ID" \
  -H "X-Webhook-Timestamp: $TS" \
  -H "X-Webhook-Signature: sha256=$SIG" \
  -d "$BODY"
```

```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{"status":"accepted","id":"evt_001"}
```

Sending exactly the same delivery again:

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{"error":{"code":"duplicate_delivery","message":"this delivery has already been received"}}
```

In Go, the signature of a delivery is:

```go
mac := hmac.New(sha256.New, secret)
mac.Write([]byte(id + "." + timestamp + "." + string(body)))
header := "sha256=" + hex.EncodeToString(mac.Sum(nil))
```

A sender that retries a delivery must keep the same `X-Webhook-Id` but send a
fresh `X-Webhook-Timestamp` and a signature computed over it.

### How a delivery is checked

The first failing step ends the request:

| Step | Check                                            | Failure                                            |
|------|--------------------------------------------------|----------------------------------------------------|
| 1    | Body size                                        | `413 payload_too_large`                            |
| 2    | Headers present and well formed                  | `401 missing_signature`, `400 missing_header`, `400 invalid_webhook_id` |
| 3    | Signature                                        | `401 invalid_signature`                            |
| 4    | Timestamp within the tolerance                   | `400 invalid_timestamp`, `401 timestamp_out_of_tolerance` |
| 5    | Payload is a JSON object with a valid `type`     | `400 invalid_payload`                              |
| 6    | Delivery ID not seen recently                    | `409 duplicate_delivery`                           |
| 7    | Processing                                       | `500 processing_failed` (the ID is released)       |

Details such as the timestamp or the payload are only reported after the
signature has been verified, so unauthenticated callers learn nothing from them.

## Errors

Every error is a JSON object with a stable `code` and a human-readable
`message`:

```sh
curl -s -X POST http://localhost:8080/webhooks \
  -H 'X-Webhook-Id: evt_002' -H "X-Webhook-Timestamp: $(date +%s)" \
  -H 'X-Webhook-Signature: sha256=0000000000000000000000000000000000000000000000000000000000000000' \
  -d '{"type":"order.created"}'
```

```json
{"error":{"code":"invalid_signature","message":"signature verification failed"}}
```

| Status | `error.code`                 | When                                                          |
|--------|------------------------------|---------------------------------------------------------------|
| 400    | `missing_header`             | `X-Webhook-Id` or `X-Webhook-Timestamp` is absent.            |
| 400    | `invalid_webhook_id`         | The ID is empty, too long or has unsupported characters.      |
| 400    | `invalid_timestamp`          | The timestamp is not Unix seconds.                            |
| 400    | `invalid_payload`            | The body is not a JSON object with a valid `type`.            |
| 401    | `missing_signature`          | `X-Webhook-Signature` is absent.                              |
| 401    | `invalid_signature`          | Wrong secret, altered body, malformed header, ...             |
| 401    | `timestamp_out_of_tolerance` | The timestamp is too old or too far in the future.            |
| 405    | _(plain text)_               | Wrong method for the path (`Allow` header is set).            |
| 409    | `duplicate_delivery`         | A delivery with this ID was already received.                 |
| 413    | `payload_too_large`          | The body is above the limit.                                  |
| 500    | `processing_failed`          | The event could not be processed; retry the delivery.         |

## Layout

```text
apis/webhook-receiver/
├── cmd/server/            wiring, configuration and graceful shutdown
├── internal/
│   ├── signature/         HMAC-SHA-256 signing and constant-time verification
│   ├── idempotency/       delivery-ID memory with TTL and an injectable clock
│   ├── replay/            timestamp tolerance
│   ├── httpapi/           POST /webhooks and JSON errors
│   └── docs/              /openapi.yaml and the Swagger UI page
├── openapi.yaml           OpenAPI 3.0.3 specification
└── spec.go                embeds openapi.yaml into the binary
```

To do real work with an accepted event, implement `httpapi.Processor` and pass it
in place of `logProcessor` in `cmd/server/main.go`.

## Tests

```sh
go test -race -count=1 ./apis/webhook-receiver/...
go vet ./apis/webhook-receiver/...
```

The suite is deterministic: the clock is injected everywhere, no test uses the
real network (only `httptest`) and nothing waits on timers. The signature code
is checked against the HMAC-SHA-256 vectors of RFC 4231. The end-to-end test in
`cmd/server/e2e_test.go` signs its requests with `crypto/hmac` directly and runs
the whole flow over HTTP: accepted deliveries, duplicates, forgeries, replays
after the tolerance window, recovery after a processing failure, the body limit
and the validation errors. Another test checks that every registered route
appears in `openapi.yaml`, and the other way around.
