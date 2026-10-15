// Package intset provides helpers for slices of integers that behave like
// sets: removing duplicates and merging sorted lists. They are good subjects
// for property-based tests, since their behavior is described by laws that
// hold for every input rather than by a few chosen examples.
package intset

// Dedup returns xs without repeated values, keeping the first occurrence of
// each value in its original position. The input is not modified.
func Dedup(xs []int) []int {
	seen := make(map[int]struct{}, len(xs))
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		if _, dup := seen[x]; dup {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// Merge combines two slices that are already sorted in ascending order into
// one sorted slice, keeping duplicates. The inputs are not modified.
func Merge(a, b []int) []int {
	out := make([]int, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] <= b[j] {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}
