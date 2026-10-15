# lru

Generic, fixed-capacity LRU cache. When it is full, the least recently used
entry is evicted. Every operation is O(1) and safe for concurrent use.

## Usage

```go
c := lru.New[string, []byte](1000)
c.OnEvict(func(k string, v []byte) { log.Println("evicted", k) })
c.Put("key", data)
if v, ok := c.Get("key"); ok {
    use(v)
}
```

- `Peek` reads without changing recency; `Remove` deletes without calling `OnEvict`.
- `Keys` lists keys from most to least recently used.

## Tests

```sh
go test -race ./pkg/lru/
```
