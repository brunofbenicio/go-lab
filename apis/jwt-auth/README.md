# JWT Auth API

A small authentication API: users register with an e-mail and a password, log
in to receive a short-lived JSON Web Token and use it as a bearer token on
protected routes. It uses only the Go standard library (`crypto/hmac`,
`crypto/pbkdf2`, `net/http` with the Go 1.22+ `ServeMux` patterns) and is
documented with OpenAPI 3.0.3.

## Features

- **HS256 tokens** implemented from scratch on `crypto/hmac`: `exp`, `nbf` and
  issuer checks with an optional leeway, constant-time signature comparison.
- **Strict verification**: tokens with `alg: none` (in any capitalization) or any
  algorithm other than HS256 are rejected before the signature is looked at, and
  tokens without an `exp` are refused.
- **Password hashing** with PBKDF2-HMAC-SHA-256 (`crypto/pbkdf2`, Go 1.24), a
  random 16-byte salt per password and 600,000 iterations. The hash is a
  self-describing string, so the cost can be raised later.
- **Login that does not leak** which e-mails exist: a wrong password and an
  unknown e-mail get the same response and cost the same effort.
- **Bearer middleware** following RFC 6750, with `WWW-Authenticate` challenges.
- Interactive documentation at `/docs` and the raw specification at
  `/openapi.yaml`.
- Graceful shutdown on `SIGINT`/`SIGTERM`.

## Running

The signing secret is read from the environment and the server refuses to start
without it. Use any random string of at least 32 bytes:

```sh
export JWT_SECRET="$(openssl rand -base64 48)"
go run ./apis/jwt-auth/cmd/server
```

| Variable     | Default    | Description                                                         |
|--------------|------------|---------------------------------------------------------------------|
| `JWT_SECRET` | _required_ | HMAC secret used to sign tokens, at least 32 bytes. Never logged.   |
| `JWT_ISSUER` | `jwt-auth` | Value of the `iss` claim. Tokens from other issuers are rejected.   |
| `TOKEN_TTL`  | `15m`      | Lifetime of access tokens, a Go duration up to `24h`.               |
| `PORT`       | `8080`     | TCP port to listen on.                                              |

Users live in memory only: restarting the server forgets every account (and
changing `JWT_SECRET` invalidates every issued token).

## Endpoints

| Method | Path             | Auth   | Description                                |
|--------|------------------|--------|--------------------------------------------|
| `POST` | `/auth/register` | no     | Create an account.                         |
| `POST` | `/auth/login`    | no     | Exchange e-mail and password for a token.  |
| `GET`  | `/me`            | bearer | The authenticated user.                    |
| `GET`  | `/openapi.yaml`  | no     | The OpenAPI document.                      |
| `GET`  | `/docs`          | no     | Swagger UI (assets loaded from a CDN).     |

### Register

```sh
curl -i -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email": "alice@example.com", "password": "correct horse battery staple", "name": "Alice Liddell"}'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{"id":"91dee8b055e2fd6290821510a96d0af3","email":"alice@example.com","name":"Alice Liddell","created_at":"2025-03-01T12:00:00.307521794Z"}
```

E-mails are compared case-insensitively. Passwords must have between 8 and 128
characters. `name` is optional (up to 100 characters).

### Log in

```sh
curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "alice@example.com", "password": "correct horse battery staple"}'
```

```json
{"access_token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJqd3QtYXV0aCIs...","token_type":"Bearer","expires_in":900}
```

The payload of the token carries `iss`, `sub` (the user ID), `iat`, `exp` and a
unique `jti`.

### Call a protected route

```sh
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "alice@example.com", "password": "correct horse battery staple"}' \
  | sed -E 's/.*"access_token":"([^"]+)".*/\1/')

curl -s http://localhost:8080/me -H "Authorization: Bearer $TOKEN"
```

```json
{"id":"91dee8b055e2fd6290821510a96d0af3","email":"alice@example.com","name":"Alice Liddell","created_at":"2025-03-01T12:00:00.307521794Z"}
```

Without a valid token the answer is `401` with a challenge:

```sh
curl -i http://localhost:8080/me
```

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Www-Authenticate: Bearer realm="api"

{"error":{"code":"missing_token","message":"authorization header is required"}}
```

## Errors

Every error is a JSON object with a stable `code` and a human-readable
`message`.

| Status | `error.code`                                   | When                                                   |
|--------|------------------------------------------------|--------------------------------------------------------|
| 400    | `invalid_json`                                 | Malformed body, unknown field or wrong type.           |
| 401    | `invalid_credentials`                          | Wrong e-mail or password on login.                     |
| 401    | `missing_token`                                | No `Authorization` header.                             |
| 401    | `invalid_request`                              | The header is not of the form `Bearer <token>`.        |
| 401    | `invalid_token`                                | Bad signature, wrong algorithm, malformed token, ...   |
| 401    | `token_expired`                                | The token's `exp` has passed.                          |
| 405    | _(plain text)_                                 | Wrong method for the path (`Allow` header is set).     |
| 409    | `email_taken`                                  | The e-mail is already registered.                      |
| 413    | `payload_too_large`                            | Request body above 4 KiB.                              |
| 415    | `unsupported_media_type`                       | `Content-Type` is not `application/json`.              |
| 422    | `invalid_email`, `invalid_password`, `invalid_name` | A registration field was rejected.                |
| 422    | `missing_credentials`                          | Login without e-mail or password.                      |
| 500    | `internal_error`                               | Unexpected failure; details are only logged.           |

## What is deliberately out of scope

This is a study project, so a few production concerns are left out on purpose:
rate limiting and lockout against password guessing, token revocation and
refresh tokens, e-mail verification, and persistent storage.

## Layout

```text
apis/jwt-auth/
├── cmd/server/            wiring, configuration and graceful shutdown
├── internal/
│   ├── jwt/               HS256 signing and verification
│   ├── password/          PBKDF2 password hashing
│   ├── users/             user model and in-memory store
│   ├── middleware/        bearer-token middleware
│   ├── httpapi/           HTTP handlers and JSON errors
│   └── docs/              /openapi.yaml and the Swagger UI page
├── openapi.yaml           OpenAPI 3.0.3 specification
└── spec.go                embeds openapi.yaml into the binary
```

## Tests

```sh
go test -race -count=1 ./apis/jwt-auth/...
go vet ./apis/jwt-auth/...
```

The suite is deterministic: the clock, the identifiers and the password cost are
injected, no test uses the real network (only `httptest`) and nothing waits on
timers. The end-to-end test in `cmd/server/e2e_test.go` runs the whole flow over
HTTP: registration, login, the protected route, forged tokens (`alg: none`,
other secrets, swapped payloads), expiry and the error responses. The token
code is also checked against the HS256 example of RFC 7515, and the password
code against PBKDF2 test vectors. Another test checks that every registered
route appears in `openapi.yaml`, and the other way around.
