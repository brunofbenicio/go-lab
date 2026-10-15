// Package sliceutil collects generic slice helpers that the standard library
// slices package does not provide: Map, Filter, Reduce, GroupBy, Chunk,
// Unique, Partition and Flatten. Every function returns a new slice and never
// modifies its input.
package sliceutil

// Map applies fn to every element and returns the results in order.
func Map[T, U any](s []T, fn func(T) U) []U {
	out := make([]U, len(s))
	for i, v := range s {
		out[i] = fn(v)
	}
	return out
}

// Filter returns the elements for which keep returns true, preserving order.
func Filter[T any](s []T, keep func(T) bool) []T {
	out := make([]T, 0, len(s))
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

// Reduce folds s into a single value, starting from init and combining each
// element with fn from left to right.
func Reduce[T, A any](s []T, init A, fn func(acc A, v T) A) A {
	acc := init
	for _, v := range s {
		acc = fn(acc, v)
	}
	return acc
}

// GroupBy buckets the elements by the key function. Within each bucket the
// original order is kept.
func GroupBy[T any, K comparable](s []T, key func(T) K) map[K][]T {
	groups := make(map[K][]T)
	for _, v := range s {
		k := key(v)
		groups[k] = append(groups[k], v)
	}
	return groups
}

// Chunk splits s into consecutive pieces of at most size elements; only the
// last one may be shorter. The pieces share memory with s but are capped so
// that appending to one never overwrites its neighbour. It panics if
// size < 1.
func Chunk[T any](s []T, size int) [][]T {
	if size < 1 {
		panic("sliceutil: Chunk size must be >= 1")
	}
	chunks := make([][]T, 0, (len(s)+size-1)/size)
	for start := 0; start < len(s); start += size {
		end := min(start+size, len(s))
		chunks = append(chunks, s[start:end:end])
	}
	return chunks
}

// Unique removes duplicates, keeping the first occurrence of each element.
func Unique[T comparable](s []T) []T {
	return UniqueBy(s, func(v T) T { return v })
}

// UniqueBy removes elements whose key was already seen, keeping the first
// occurrence of each key.
func UniqueBy[T any, K comparable](s []T, key func(T) K) []T {
	seen := make(map[K]struct{}, len(s))
	out := make([]T, 0, len(s))
	for _, v := range s {
		k := key(v)
		if _, dup := seen[k]; !dup {
			seen[k] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// Partition splits s into the elements that satisfy pred and those that do
// not, preserving order in both.
func Partition[T any](s []T, pred func(T) bool) (yes, no []T) {
	for _, v := range s {
		if pred(v) {
			yes = append(yes, v)
		} else {
			no = append(no, v)
		}
	}
	return yes, no
}

// Flatten concatenates the inner slices into one.
func Flatten[T any](ss [][]T) []T {
	n := 0
	for _, s := range ss {
		n += len(s)
	}
	out := make([]T, 0, n)
	for _, s := range ss {
		out = append(out, s...)
	}
	return out
}
