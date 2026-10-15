# bloom

Bloom filter: a compact probabilistic set that never produces false negatives
and produces false positives with a configurable probability. Good as a cheap
pre-check before an expensive lookup.

## Usage

```go
f := bloom.New(10_000, 0.01) // ~10k elements, 1% false positives
f.AddString("alice@example.com")

if f.MayContainString(email) {
    // might exist: confirm in the database
} else {
    // definitely does not exist: skip the lookup
}
```

Hashing uses FNV-1a with double hashing. Elements cannot be removed, and a
`Filter` is not safe for concurrent use.

## Tests

```sh
go test -race ./pkg/bloom/
```
