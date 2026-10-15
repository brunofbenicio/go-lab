# ringbuffer

Generic fixed-capacity circular buffer that overwrites the oldest element when
full. Ideal for "keep only the last N" cases such as recent log lines or
metric samples. Safe for concurrent use.

## Usage

```go
recent := ringbuffer.New[string](100)
recent.Push(line)           // overwrites the oldest line once full
lines := recent.Items()     // oldest to newest
tail := recent.Last(10)     // the 10 newest
```

`Push` returns the element that was overwritten, if any.

## Tests

```sh
go test -race ./pkg/ringbuffer/
```
