# 006 - Maximum subarray (Kadane)

## Problem

Given a slice of integers, find the non-empty contiguous subarray with the
largest sum and return the sum together with the start and end indices
(inclusive). Do it in a single pass.

`MaxSubarray(xs []int) (Result, bool)` returns `false` for an empty slice. If
all numbers are negative, the answer is the largest single element.

## Examples

```go
kadane.MaxSubarray([]int{-2, 1, -3, 4, -1, 2, 1, -5, 4})
// Result{Sum: 6, Start: 3, End: 6}  -> [4, -1, 2, 1]
```

## Concepts

Dynamic programming with constant memory, and testing against a brute-force
oracle fed by a seeded random source.

## Running the tests

```sh
go test -race ./exercises/006-max-subarray/
```
