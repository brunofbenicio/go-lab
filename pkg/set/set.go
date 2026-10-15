// Package set provides a generic set type backed by a map, with the usual
// algebra operations (union, intersection, difference).
//
// A Set is not safe for concurrent use; guard it with a mutex if it is shared
// between goroutines.
package set

// Set is a collection of unique elements. The zero value is a nil set: it can
// be read, but Add on it panics, so create sets with New.
type Set[T comparable] map[T]struct{}

// New returns a set containing items.
func New[T comparable](items ...T) Set[T] {
	s := make(Set[T], len(items))
	s.Add(items...)
	return s
}

// Add inserts items into the set.
func (s Set[T]) Add(items ...T) {
	for _, it := range items {
		s[it] = struct{}{}
	}
}

// Remove deletes items from the set. Missing items are ignored.
func (s Set[T]) Remove(items ...T) {
	for _, it := range items {
		delete(s, it)
	}
}

// Has reports whether item is in the set.
func (s Set[T]) Has(item T) bool {
	_, ok := s[item]
	return ok
}

// Len returns the number of elements.
func (s Set[T]) Len() int { return len(s) }

// Clone returns an independent copy of the set.
func (s Set[T]) Clone() Set[T] {
	c := make(Set[T], len(s))
	for it := range s {
		c[it] = struct{}{}
	}
	return c
}

// Slice returns the elements in no particular order.
func (s Set[T]) Slice() []T {
	out := make([]T, 0, len(s))
	for it := range s {
		out = append(out, it)
	}
	return out
}

// Union returns a new set with the elements of s and o.
func (s Set[T]) Union(o Set[T]) Set[T] {
	u := s.Clone()
	for it := range o {
		u[it] = struct{}{}
	}
	return u
}

// Intersect returns a new set with the elements present in both s and o.
func (s Set[T]) Intersect(o Set[T]) Set[T] {
	r := make(Set[T])
	for it := range s {
		if o.Has(it) {
			r[it] = struct{}{}
		}
	}
	return r
}

// Difference returns a new set with the elements of s that are not in o.
func (s Set[T]) Difference(o Set[T]) Set[T] {
	r := make(Set[T])
	for it := range s {
		if !o.Has(it) {
			r[it] = struct{}{}
		}
	}
	return r
}

// SymmetricDifference returns the elements that are in exactly one of s and o.
func (s Set[T]) SymmetricDifference(o Set[T]) Set[T] {
	return s.Difference(o).Union(o.Difference(s))
}

// IsSubset reports whether every element of s is also in o.
func (s Set[T]) IsSubset(o Set[T]) bool {
	for it := range s {
		if !o.Has(it) {
			return false
		}
	}
	return true
}

// Equal reports whether s and o contain exactly the same elements.
func (s Set[T]) Equal(o Set[T]) bool {
	return len(s) == len(o) && s.IsSubset(o)
}
