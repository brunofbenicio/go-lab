# multierr

Combines several errors into one that still works with `errors.Is` and
`errors.As`. Compared with `errors.Join`, it flattens nested combinations,
returns a lone error unchanged, ignores `nil`, prints a count and helps with
the "do not lose the cleanup error" pattern.

## Usage

```go
err := multierr.Combine(validateName(), validateAge(), validateEmail())

func process() (err error) {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer func() { multierr.AppendInto(&err, f.Close()) }()
    return read(f)
}

for _, e := range multierr.Errors(err) { log.Println(e) }
```

## Tests

```sh
go test -race ./pkg/multierr/
```
