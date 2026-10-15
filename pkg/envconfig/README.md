# envconfig

Fills a struct from environment variables using struct tags, with defaults,
required fields and nested structs. Every problem is collected and reported at
once, instead of failing on the first one.

## Usage

```go
type Config struct {
    Port    int           `env:"PORT" default:"8080"`
    Timeout time.Duration `env:"TIMEOUT" default:"5s"`
    Hosts   []string      `env:"HOSTS"` // comma-separated
    DSN     string        `env:"DSN" required:"true"`
}

var cfg Config
if err := envconfig.Load(&cfg, envconfig.WithPrefix("APP_")); err != nil {
    log.Fatal(err)
}
```

Supports strings, bools, ints, uints, floats, `time.Duration`, slices and
nested structs (whose `env` tag acts as a prefix). `WithLookup` swaps
`os.LookupEnv` for any source, which keeps tests hermetic.

## Tests

```sh
go test -race ./pkg/envconfig/
```
