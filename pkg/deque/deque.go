// Package deque implements a generic double-ended queue on top of a growing
// circular buffer. Pushing and popping at either end is amortized O(1).
//
// A Deque is not safe for concurrent use. Its zero value is an empty deque
// ready to use.
package deque

// Deque is a double-ended queue.
type Deque[T any] struct {
	buf  []T
	head int // index of the first element
	size int
}

// Len returns the number of elements.
func (d *Deque[T]) Len() int { return d.size }

// PushBack appends v at the back.
func (d *Deque[T]) PushBack(v T) {
	d.growIfFull()
	d.buf[(d.head+d.size)%len(d.buf)] = v
	d.size++
}

// PushFront inserts v at the front.
func (d *Deque[T]) PushFront(v T) {
	d.growIfFull()
	d.head = (d.head - 1 + len(d.buf)) % len(d.buf)
	d.buf[d.head] = v
	d.size++
}

// PopFront removes and returns the first element. The second result is false
// if the deque is empty.
func (d *Deque[T]) PopFront() (T, bool) {
	var zero T
	if d.size == 0 {
		return zero, false
	}
	v := d.buf[d.head]
	d.buf[d.head] = zero // do not retain references
	d.head = (d.head + 1) % len(d.buf)
	d.size--
	return v, true
}

// PopBack removes and returns the last element. The second result is false if
// the deque is empty.
func (d *Deque[T]) PopBack() (T, bool) {
	var zero T
	if d.size == 0 {
		return zero, false
	}
	i := (d.head + d.size - 1) % len(d.buf)
	v := d.buf[i]
	d.buf[i] = zero
	d.size--
	return v, true
}

// Front returns the first element without removing it.
func (d *Deque[T]) Front() (T, bool) { return d.At(0) }

// Back returns the last element without removing it.
func (d *Deque[T]) Back() (T, bool) { return d.At(d.size - 1) }

// At returns the i-th element counting from the front. The second result is
// false if i is out of range.
func (d *Deque[T]) At(i int) (T, bool) {
	if i < 0 || i >= d.size {
		var zero T
		return zero, false
	}
	return d.buf[(d.head+i)%len(d.buf)], true
}

// Slice returns a copy of the elements ordered from front to back.
func (d *Deque[T]) Slice() []T {
	out := make([]T, d.size)
	for i := range out {
		out[i] = d.buf[(d.head+i)%len(d.buf)]
	}
	return out
}

func (d *Deque[T]) growIfFull() {
	if d.size < len(d.buf) {
		return
	}
	nb := make([]T, max(4, 2*len(d.buf)))
	copy(nb, d.Slice())
	d.buf, d.head = nb, 0
}
