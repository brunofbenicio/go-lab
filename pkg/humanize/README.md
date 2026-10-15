# humanize

Formats numbers for people: byte sizes in SI and IEC units, compact durations
and integers with thousands separators. Handles negatives, the extremes of
`int64` and rounding that crosses a unit boundary.

## Usage

```go
humanize.Bytes(1_500_000)          // "1.5 MB"   (base 1000)
humanize.IBytes(3 << 20)           // "3 MiB"    (base 1024)
humanize.Duration(36 * time.Hour)  // "1d 12h"
humanize.Thousands(1234567, ".")   // "1.234.567"
```

## Tests

```sh
go test -race ./pkg/humanize/
```
