# 008 - Two sum

## Problem

- `TwoSum(nums []int, target int) (i, j int, ok bool)` returns the indices
  `i < j` of two elements that add up to `target`, in a single pass. An element
  cannot be used twice.
- `Pairs(nums []int, target int) [][2]int` returns every distinct pair of
  values `{a, b}` (with `a <= b`) that adds up to `target`, sorted by `a`.

## Examples

```go
twosum.TwoSum([]int{2, 7, 11, 15}, 9)           // 0, 1, true
twosum.Pairs([]int{1, 5, 7, -1, 5, 3, 4}, 6)    // [[-1 7] [1 5]]
```

## Concepts

Trading memory for time with a hash map, looking up the complement instead of
searching for it, and returning deterministic results from a map-based count.

## Running the tests

```sh
go test -race ./exercises/008-two-sum/
```
