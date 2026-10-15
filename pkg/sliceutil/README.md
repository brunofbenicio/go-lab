# sliceutil

Generic slice helpers that complement the standard `slices` package: `Map`,
`Filter`, `Reduce`, `GroupBy`, `Chunk`, `Unique`/`UniqueBy`, `Partition` and
`Flatten`. Each one returns a new slice and never modifies its input.

## Usage

```go
names := sliceutil.Map(users, func(u User) string { return u.Name })
adults := sliceutil.Filter(users, func(u User) bool { return u.Age >= 18 })
byCity := sliceutil.GroupBy(users, func(u User) string { return u.City })
batches := sliceutil.Chunk(ids, 100) // e.g. bulk inserts in batches of 100
```

`Chunk` panics when `size < 1`, like `make` with a negative length.

## Tests

```sh
go test -race ./pkg/sliceutil/
```
