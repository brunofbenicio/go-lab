# consistenthash

Consistent hash ring with virtual nodes. It maps keys to nodes so that adding
or removing a node moves only a small share of the keys, which suits sharded
caches, partitioned workers and client-side load balancing.

## Usage

```go
r := consistenthash.New(100) // 100 virtual nodes per node
r.Add("cache-1", "cache-2", "cache-3")

node, ok := r.Get("user:42")
r.Remove("cache-2") // only keys that lived on cache-2 move
```

The hash function is replaceable with `WithHash`. The ring is safe for
concurrent use.

## Tests

```sh
go test -race ./pkg/consistenthash/
```
