# truncate

Rune-aware string shortening. It never splits a multi-byte character, so
accented text and emoji stay valid UTF-8.

## Usage

```go
truncate.String("Ação rápida e segura", 10, "…")        // "Ação rápi…"
truncate.Words("the quick brown fox", 15, "…")          // "the quick…"
truncate.Middle("quarterly-report-final.pdf", 14, "…")  // "quarter…al.pdf"
truncate.Bytes("ação", 3)                               // "aç" (fits a 3-byte column)
```

The limit passed to `String`, `Words` and `Middle` counts the suffix or
ellipsis as well. Runes are the unit, not grapheme clusters.

## Tests

```sh
go test -race ./pkg/truncate/
```
