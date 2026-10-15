# set

Generic set (`Set[T comparable]`) backed by a map, with the usual algebra:
union, intersection, difference, symmetric difference, subset and equality.

## Usage

```go
admins := set.New("ana", "bia")
online := set.New("bia", "caio")

admins.Intersect(online).Has("bia") // true
online.Difference(admins).Slice()   // ["caio"] (order is unspecified)
```

Operations return new sets and never modify their operands. A `Set` is not
safe for concurrent use.

## Tests

```sh
go test -race ./pkg/set/
```
