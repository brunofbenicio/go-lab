# slugify

Turns text into URL-friendly slugs: lowercase ASCII letters and digits joined
by a single separator. Latin accents (including Portuguese) are transliterated
without external dependencies.

## Usage

```go
slugify.Make("Ação e Reação!")                         // "acao-e-reacao"
slugify.Make("Hello World", slugify.WithSeparator("_")) // "hello_world"
slugify.Make("The quick brown fox", slugify.WithMaxLength(12)) // "the-quick"
```

Symbols and other scripts become separators, apostrophes are dropped, and
decomposed (NFD) accents are handled. `Make` is idempotent.

## Tests

```sh
go test -race ./pkg/slugify/
```
