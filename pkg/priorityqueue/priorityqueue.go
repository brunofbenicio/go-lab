// Package priorityqueue provides a generic priority queue built on
// container/heap. Ordering is defined by a user-supplied less function, so
// the same type works as a min-heap, a max-heap or a heap of structs.
//
// A Queue is not safe for concurrent use. Elements with equal priority are
// returned in unspecified order.
package priorityqueue

import "container/heap"

// Queue is a priority queue. Pop always returns the element that sorts first
// according to the less function given to New.
type Queue[T any] struct{ h items[T] }

// New creates a queue ordered by less, which must report whether a should be
// popped before b. Optional initial items are heapified in O(n).
func New[T any](less func(a, b T) bool, initial ...T) *Queue[T] {
	q := &Queue[T]{h: items[T]{data: append([]T(nil), initial...), less: less}}
	heap.Init(&q.h)
	return q
}

// Push adds v to the queue in O(log n).
func (q *Queue[T]) Push(v T) { heap.Push(&q.h, v) }

// Pop removes and returns the first element in O(log n). The second result is
// false if the queue is empty.
func (q *Queue[T]) Pop() (T, bool) {
	if len(q.h.data) == 0 {
		var zero T
		return zero, false
	}
	return heap.Pop(&q.h).(T), true
}

// Peek returns the first element without removing it.
func (q *Queue[T]) Peek() (T, bool) {
	if len(q.h.data) == 0 {
		var zero T
		return zero, false
	}
	return q.h.data[0], true
}

// Len returns the number of queued elements.
func (q *Queue[T]) Len() int { return len(q.h.data) }

// items adapts a slice to heap.Interface.
type items[T any] struct {
	data []T
	less func(a, b T) bool
}

func (s items[T]) Len() int           { return len(s.data) }
func (s items[T]) Less(i, j int) bool { return s.less(s.data[i], s.data[j]) }
func (s items[T]) Swap(i, j int)      { s.data[i], s.data[j] = s.data[j], s.data[i] }
func (s *items[T]) Push(x any)        { s.data = append(s.data, x.(T)) }
func (s *items[T]) Pop() any {
	n := len(s.data) - 1
	v := s.data[n]
	var zero T
	s.data[n] = zero // do not retain references
	s.data = s.data[:n]
	return v
}
