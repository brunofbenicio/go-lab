# pagination

Offset-based pagination math: SQL `OFFSET`/`LIMIT`, total pages, next/previous
flags, the displayed item range and a window of page numbers for a pager. It
sanitizes bad input (page below 1, absurd page sizes) instead of failing.

## Usage

```go
p := pagination.FromQuery(r.URL.Query(), total) // ?page=3&per_page=20
rows, _ := db.Query("SELECT ... LIMIT ? OFFSET ?", p.Limit(), p.Offset())

from, to := p.Range()  // "showing 41-60 of 95"
pages := p.Window(5)   // [1 2 3 4 5] around the current page
part := pagination.Slice(items, p) // paginate an in-memory slice
```

## Tests

```sh
go test -race ./pkg/pagination/
```
