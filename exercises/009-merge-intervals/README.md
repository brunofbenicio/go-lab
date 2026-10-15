# 009 - Merge intervals

## Problem

Given a list of closed integer intervals `[start, end]`, merge the ones that
overlap or touch and return a sorted list of disjoint intervals.

`Merge(in []Interval) ([]Interval, error)` must not modify its input and returns
`ErrInvalidInterval` when an interval has `Start > End`.

## Examples

```go
intervals.Merge([]intervals.Interval{{1, 3}, {2, 6}, {8, 10}, {15, 18}})
// [{1 6} {8 10} {15 18}]
```

## Concepts

Sort first, then sweep once while extending the last interval; cloning input to
avoid side effects; `slices.SortFunc` with `cmp.Or`.

## Running the tests

```sh
go test -race ./exercises/009-merge-intervals/
```
