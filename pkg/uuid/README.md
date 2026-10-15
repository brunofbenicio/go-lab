# uuid

Creates and parses version 4 (random) UUIDs per RFC 4122 using only
`crypto/rand`. `UUID` is a comparable `[16]byte`, usable as a map key, and it
serializes as a string in JSON.

## Usage

```go
id := uuid.New()           // 3f1c9a5e-6b2d-4c8e-9a47-0d2e5b7f1a90
s := id.String()           // canonical lowercase form
back, err := uuid.Parse(s) // accepts upper or lower case
```

`NewFromReader` takes any `io.Reader`, which makes creation deterministic in
tests.

## Tests

```sh
go test -race ./pkg/uuid/
```
