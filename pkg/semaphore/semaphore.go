// Package semaphore implements a counting semaphore that limits how many
// goroutines may run a section of code at once, with context-aware
// acquisition.
package semaphore

import "context"

// Semaphore allows at most n concurrent holders. It is safe for concurrent
// use.
type Semaphore struct{ slots chan struct{} }

// New creates a semaphore with n slots. It panics if n < 1.
func New(n int) *Semaphore {
	if n < 1 {
		panic("semaphore: n must be >= 1")
	}
	return &Semaphore{slots: make(chan struct{}, n)}
}

// Acquire takes a slot, blocking until one is free or ctx is done, in which
// case it returns ctx's error and takes nothing. A context that is already
// done never acquires.
func (s *Semaphore) Acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire takes a slot if one is free right now and reports whether it did.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release frees a slot. It panics if there is no matching Acquire.
func (s *Semaphore) Release() {
	select {
	case <-s.slots:
	default:
		panic("semaphore: Release without a matching Acquire")
	}
}

// Do acquires a slot, runs fn and releases the slot, even if fn panics.
func (s *Semaphore) Do(ctx context.Context, fn func() error) error {
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()
	return fn()
}

// InUse returns how many slots are currently held.
func (s *Semaphore) InUse() int { return len(s.slots) }

// Cap returns the total number of slots.
func (s *Semaphore) Cap() int { return cap(s.slots) }
