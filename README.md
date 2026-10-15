# go-lab

A personal Go lab: small, tested, reusable building blocks, each one landed
through its own pull request.

## Layout

| Path | What's inside |
|------|---------------|
| `apis/` | REST API examples built with the standard library only, each with an OpenAPI spec and a Swagger UI page (`todo-api`, `url-shortener`, `jwt-auth`, `webhook-receiver`) |
| `pkg/` | Reusable libraries: retry, rate limiting, caches, generic collections, validation and more |
| `testing-examples/` | Focused examples of Go testing techniques: table-driven tests, fuzzing, golden files, `httptest`, fakes and others |
| `exercises/` | Katas with solutions and tests |
| `cmd/` | Small command-line tools |

Every directory is an independent package with its own tests and, where it
helps, its own README.

## Running

```sh
go test ./...          # everything
go test -race ./...    # with the race detector
go vet ./...           # static analysis
```

Run one of the APIs and open its Swagger UI:

```sh
PORT=8080 go run ./apis/todo-api/cmd/server
# http://localhost:8080/docs
```

## Requirements

- Go 1.24 or newer
- No third-party dependencies

## License

[MIT](LICENSE)
