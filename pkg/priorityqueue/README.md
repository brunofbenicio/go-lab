# priorityqueue

Generic priority queue on top of `container/heap`. A `less` function defines
the order, so the same type serves as a min-heap, a max-heap or a heap of
structs, without writing `heap.Interface` boilerplate.

## Usage

```go
q := priorityqueue.New(func(a, b Job) bool { return a.Priority > b.Priority })
q.Push(job)
next, ok := q.Pop() // highest priority first
```

`Push` and `Pop` are O(log n); `Peek` and `Len` are O(1). Equal priorities pop
in unspecified order. Not safe for concurrent use.

## Tests

```sh
go test -race ./pkg/priorityqueue/
```
