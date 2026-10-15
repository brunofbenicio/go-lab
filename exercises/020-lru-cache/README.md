# 020 - LRU cache

## Problem

Build a generic least-recently-used cache, `Cache[K comparable, V any]`, with
constant-time operations:

- `New(capacity, onEvict)` returns `ErrCapacity` for non-positive capacities.
  `onEvict` (optional) is called for every entry removed to make room.
- `Get` returns a value and marks it most recently used; `Peek` does not.
- `Put` inserts or updates a key, evicting the least recently used entry when
  the cache is full.
- `Delete`, `Len` and `Keys` (most recent first).

## Examples

```go
c, _ := lru.New[string, int](2, nil)
c.Put("a", 1)
c.Put("b", 2)
c.Get("a")      // refreshes "a"
c.Put("c", 3)   // evicts "b"
c.Keys()        // [c a]
```

## Concepts

A hash map plus a doubly linked list (`container/list`), generics with two type
parameters, and storing callbacks in a struct.

## Running the tests

```sh
go test -race ./exercises/020-lru-cache/
```
