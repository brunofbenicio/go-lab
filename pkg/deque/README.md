# deque

Generic double-ended queue built on a growing circular buffer. Push and pop at
either end are amortized O(1), and the zero value is ready to use. It works as
a queue, a stack or a sliding-window helper.

## Usage

```go
var d deque.Deque[string]
d.PushBack("job-1")
d.PushFront("urgent")
next, _ := d.PopFront() // "urgent"
```

A `Deque` is not safe for concurrent use.

## Tests

```sh
go test -race ./pkg/deque/
```
