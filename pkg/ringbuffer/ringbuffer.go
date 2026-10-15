// Package ringbuffer implements a fixed-capacity circular buffer that
// overwrites the oldest element when full. It is the natural structure for
// "keep only the last N items" such as recent log lines or metric samples.
package ringbuffer

import "sync"

// Buffer is a circular buffer safe for concurrent use.
type Buffer[T any] struct {
	mu    sync.Mutex
	buf   []T
	start int // index of the oldest element
	size  int
}

// New creates a buffer that keeps the last capacity elements. It panics if
// capacity < 1.
func New[T any](capacity int) *Buffer[T] {
	if capacity < 1 {
		panic("ringbuffer: capacity must be >= 1")
	}
	return &Buffer[T]{buf: make([]T, capacity)}
}

// Push appends v. If the buffer was full, the oldest element is overwritten
// and returned with ok set to true.
func (b *Buffer[T]) Push(v T) (evicted T, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size < len(b.buf) {
		b.buf[(b.start+b.size)%len(b.buf)] = v
		b.size++
		return evicted, false
	}
	evicted = b.buf[b.start]
	b.buf[b.start] = v
	b.start = (b.start + 1) % len(b.buf)
	return evicted, true
}

// Items returns the stored elements from oldest to newest.
func (b *Buffer[T]) Items() []T {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.last(b.size)
}

// Last returns up to n of the newest elements, ordered from oldest to newest.
func (b *Buffer[T]) Last(n int) []T {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.last(n)
}

// last requires b.mu to be held.
func (b *Buffer[T]) last(n int) []T {
	n = max(0, min(n, b.size))
	out := make([]T, n)
	for i := range out {
		out[i] = b.buf[(b.start+b.size-n+i)%len(b.buf)]
	}
	return out
}

// Len returns the number of stored elements.
func (b *Buffer[T]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

// Cap returns the maximum number of elements the buffer keeps.
func (b *Buffer[T]) Cap() int { return len(b.buf) }

// Full reports whether the next Push will overwrite an element.
func (b *Buffer[T]) Full() bool { return b.Len() == len(b.buf) }

// Reset discards all elements.
func (b *Buffer[T]) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.buf)
	b.start, b.size = 0, 0
}
