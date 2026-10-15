# Benchmarks with `b.ReportAllocs` and sub-benchmarks

## What it solves

"Is this faster?" needs a measurement, not a guess. Go benchmarks run a
function many times, adjust the iteration count automatically and report time
and allocations per operation. Comparing implementations side by side, at
several input sizes, shows where an optimization really pays off.

## When to use

- Before and after an optimization, to prove it helped.
- To compare two designs (builder versus concatenation, map versus slice).
- To keep an allocation budget with `testing.AllocsPerRun` as a normal test.

## What this example shows

- Three implementations of the same function plus a test that they agree.
- `BenchmarkJoin`: sub-benchmarks (`b.Run`) for each implementation and size,
  with `b.ReportAllocs()` and `b.SetBytes()`.
- A package-level `sink`, so the compiler cannot delete the work being measured.
- `BenchmarkJoinLoop`: the Go 1.24 `b.Loop()` style, which needs no sink and
  no `b.ResetTimer()` (setup before the loop is not measured).
- `BenchmarkJoinParallel`: `b.RunParallel` for concurrent throughput.
- `testing.AllocsPerRun` guarding "at most two allocations".

## Running

```sh
go test ./testing-examples/benchmarks/                          # tests only
go test -run xxx -bench . -benchmem ./testing-examples/benchmarks/
go test -run xxx -bench 'Join/' -benchtime 100x ./testing-examples/benchmarks/   # fixed iteration count
go test -run xxx -bench . -count 6 ./testing-examples/benchmarks/ > old.txt      # then compare runs with benchstat
```

`-run xxx` matches no test, so only benchmarks run. Numbers vary between
machines; compare runs taken on the same machine and look at the ratios.
When an expensive setup lives inside a classic `for i := 0; i < b.N; i++`
benchmark, call `b.ResetTimer()` after it.
