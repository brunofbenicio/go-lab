# workerpool

Two ways to bound concurrency:

- `Pool`: a fixed set of workers with a task queue; `Submit` blocks (honoring
  the context) when the queue is full and `Close` drains everything.
- `Map`: parallel transformation of a slice that keeps input order, cancels the
  remaining work on the first error and returns that error.

## Usage

```go
p := workerpool.New(4, 100)
defer p.Close()
_ = p.Submit(ctx, func() { process(job) })

sizes, err := workerpool.Map(ctx, 8, urls, func(ctx context.Context, u string) (int, error) {
    return fetchSize(ctx, u)
})
```

## Tests

```sh
go test -race ./pkg/workerpool/
```
