// Package binsearch implements binary search over sorted slices using generics.
package binsearch

import "cmp"

// LowerBound returns the first index i such that xs[i] >= target, or
// len(xs) when every element is smaller. xs must be sorted ascending.
func LowerBound[T cmp.Ordered](xs []T, target T) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := lo + (hi-lo)/2 // avoids overflow of lo+hi
		if xs[mid] < target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// UpperBound returns the first index i such that xs[i] > target, or
// len(xs) when no element is greater. xs must be sorted ascending.
func UpperBound[T cmp.Ordered](xs []T, target T) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if xs[mid] <= target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// Search returns the index of the first occurrence of target and true, or
// the insertion point and false when target is absent.
func Search[T cmp.Ordered](xs []T, target T) (int, bool) {
	i := LowerBound(xs, target)
	return i, i < len(xs) && xs[i] == target
}

// Count returns how many times target appears in the sorted slice xs.
func Count[T cmp.Ordered](xs []T, target T) int {
	return UpperBound(xs, target) - LowerBound(xs, target)
}
