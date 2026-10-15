# Data races and `go test -race`

## What it solves

A data race (two goroutines touching the same memory, at least one writing,
without synchronization) causes bugs that appear once in a thousand runs and
vanish when you add a `Println`. The race detector instruments the program and
reports the exact lines of conflicting accesses the moment they happen.

## When to use

- Always in CI: `go test -race ./...`.
- For any type documented as safe for concurrent use.
- After adding goroutines, shared maps, caches or lazily initialized state.

## What this example shows

The package contains the **fixed** versions: a `Counter` built on
`atomic.Int64`, and a `Registry` whose map is guarded by an `RWMutex` with a
double-checked creation. The tests make many goroutines hit them simultaneously
(`WaitGroup` for synchronization, no sleeps) and check both the absence of races
and the exactness of the results.

The classic bug these types avoid:

```go
type Counter struct{ n int64 }

func (c *Counter) Inc() { c.n++ } // read-modify-write: NOT atomic
```

With that version, `go test -race` fails with something like:

```
WARNING: DATA RACE
Read at 0x00c00011c228 by goroutine 21:
  metrics.(*Counter).Inc()
      metrics.go:16
Previous write at 0x00c00011c228 by goroutine 8:
  metrics.(*Counter).Inc()
      metrics.go:16
...
--- FAIL: TestCounterConcurrentIncrements
    testing.go: race detected during execution of test
```

and, even without `-race`, the final value would often be lower than expected
because increments are lost.

## Tips

- The detector finds races that *happen* during the run, so tests must create
  real contention; run them more than once with `-count`.
- `-race` slows execution 5-10x and needs cgo support in the toolchain.
- Prefer sharing by communicating (channels) or confining data to one
  goroutine; use mutexes and atomics where shared state is really needed.

## Running

```sh
go test -race -count=1 ./testing-examples/race-detector/
go test -race -count=20 ./testing-examples/race-detector/   # more chances to catch a race
```
