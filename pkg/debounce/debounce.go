// Package debounce delays a function until calls stop arriving: every call
// restarts the wait, and the function runs once, wait after the last call.
// It is the right tool for "save after the user stops typing" or "reload
// after the burst of file events ends".
//
// The timer is injectable (WithAfterFunc), so tests need no real waiting.
package debounce

import (
	"sync"
	"time"
)

// Stopper is the part of a timer that Debouncer needs. *time.Timer satisfies
// it.
type Stopper interface{ Stop() bool }

// AfterFunc schedules f to run after d, like time.AfterFunc.
type AfterFunc func(d time.Duration, f func()) Stopper

// Option customizes a Debouncer.
type Option func(*Debouncer)

// WithAfterFunc replaces the timer factory (default: time.AfterFunc).
func WithAfterFunc(after AfterFunc) Option { return func(d *Debouncer) { d.after = after } }

// Debouncer coalesces bursts of Call into a single execution of fn. It is
// safe for concurrent use.
type Debouncer struct {
	mu      sync.Mutex
	wait    time.Duration
	fn      func()
	after   AfterFunc
	timer   Stopper
	gen     uint64 // invalidates timers that were stopped too late
	pending bool
}

// New creates a Debouncer that runs fn once wait has elapsed since the last
// Call.
func New(wait time.Duration, fn func(), opts ...Option) *Debouncer {
	d := &Debouncer{wait: wait, fn: fn, after: func(d time.Duration, f func()) Stopper {
		return time.AfterFunc(d, f)
	}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Call (re)starts the wait. fn runs in its own goroutine when it expires.
func (d *Debouncer) Call() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stop()
	gen := d.gen
	d.pending = true
	d.timer = d.after(d.wait, func() { d.fire(gen) })
}

// Cancel drops the pending execution, if any, and reports whether there was
// one.
func (d *Debouncer) Cancel() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	was := d.pending
	d.stop()
	return was
}

// Flush runs fn immediately if an execution is pending, in the caller's
// goroutine, and reports whether it did.
func (d *Debouncer) Flush() bool {
	d.mu.Lock()
	was := d.pending
	d.stop()
	d.mu.Unlock()
	if was {
		d.fn()
	}
	return was
}

// stop invalidates the current timer. It requires d.mu to be held.
func (d *Debouncer) stop() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.gen++
	d.pending = false
}

func (d *Debouncer) fire(gen uint64) {
	d.mu.Lock()
	if gen != d.gen || !d.pending {
		d.mu.Unlock()
		return // superseded by a later Call, Cancel or Flush
	}
	d.pending = false
	d.mu.Unlock()
	d.fn()
}
