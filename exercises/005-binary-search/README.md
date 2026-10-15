# 005 - Binary search

## Problem

Implement binary search over sorted slices with generics (`cmp.Ordered`):

- `LowerBound` returns the first index whose element is `>= target`.
- `UpperBound` returns the first index whose element is `> target`.
- `Search` returns the index of the first occurrence and whether it was found
  (or the insertion point when absent).
- `Count` returns how many times a value occurs, in `O(log n)`.

## Examples

```go
xs := []int{2, 2, 4, 4, 4, 8}
binsearch.Search(xs, 4)      // 2, true
binsearch.Search(xs, 5)      // 5, false
binsearch.Count(xs, 4)       // 3
```

## Concepts

Half-open intervals, avoiding `lo+hi` overflow, handling duplicates, and
generic constraints.

## Running the tests

```sh
go test -race ./exercises/005-binary-search/
```
