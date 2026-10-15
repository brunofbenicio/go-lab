# singleflight

Suppresses duplicate concurrent work: while a call for a key is running, other
callers with the same key wait and share its result instead of repeating it.
Prevents a cache stampede when a hot entry expires.

## Usage

```go
var g singleflight.Group[string, *User]

user, shared, err := g.Do(id, func() (*User, error) {
    return db.LoadUser(id) // runs once even with 1000 concurrent callers
})
```

- Generic in key and value; the zero value of `Group` is ready to use.
- Errors are shared too; if the function panics, waiting callers receive
  `ErrPanic` and the panic propagates in the goroutine that ran it.

## Tests

```sh
go test -race ./pkg/singleflight/
```
