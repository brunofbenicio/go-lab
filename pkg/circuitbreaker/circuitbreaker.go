// Package circuitbreaker implements the circuit breaker pattern: after too
// many consecutive failures it "opens" and rejects calls immediately, giving a
// failing dependency time to recover, then lets a single trial call through.
//
// The clock is injectable, so state transitions are testable without waiting.
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// State is the state of a Breaker.
type State int

// Possible states.
const (
	Closed   State = iota // calls flow normally
	Open                  // calls are rejected with ErrOpen
	HalfOpen              // one trial call is allowed to probe recovery
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	}
	return "unknown"
}

// ErrOpen is returned when a call is rejected because the circuit is open.
var ErrOpen = errors.New("circuitbreaker: circuit is open")

var errPanic = errors.New("circuitbreaker: call panicked")

// Option customizes a Breaker.
type Option func(*Breaker)

// WithThreshold sets how many consecutive failures open the circuit
// (default 5).
func WithThreshold(n int) Option { return func(b *Breaker) { b.threshold = max(n, 1) } }

// WithCooldown sets how long the circuit stays open before a trial call is
// allowed (default 30s).
func WithCooldown(d time.Duration) Option { return func(b *Breaker) { b.cooldown = d } }

// WithClock replaces the time source (default: time.Now).
func WithClock(now func() time.Time) Option { return func(b *Breaker) { b.now = now } }

// OnStateChange registers a callback invoked after every transition. It runs
// outside the breaker's lock, so it may call State.
func OnStateChange(fn func(from, to State)) Option { return func(b *Breaker) { b.onChange = fn } }

// Breaker guards calls to an unreliable dependency. It is safe for concurrent
// use.
type Breaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	now       func() time.Time
	onChange  func(from, to State)

	state    State
	failures int
	openedAt time.Time
	pending  [][2]State // transitions to report after unlocking
}

// New creates a closed Breaker.
func New(opts ...Option) *Breaker {
	b := &Breaker{threshold: 5, cooldown: 30 * time.Second, now: time.Now}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Execute runs fn unless the circuit is open, in which case it returns
// ErrOpen without calling fn. A non-nil error (or a panic) from fn counts as
// a failure.
func (b *Breaker) Execute(fn func() error) error {
	if err := b.allow(); err != nil {
		return err
	}
	finished := false
	defer func() {
		if !finished {
			b.record(errPanic)
		}
	}()
	err := fn()
	finished = true
	b.record(err)
	return err
}

// State reports the current state. An open circuit whose cooldown has elapsed
// is reported as HalfOpen, since the next call will be a trial.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == Open && b.now().Sub(b.openedAt) >= b.cooldown {
		return HalfOpen
	}
	return b.state
}

func (b *Breaker) allow() error {
	b.mu.Lock()
	defer b.unlock()
	switch b.state {
	case Open:
		if b.now().Sub(b.openedAt) < b.cooldown {
			return ErrOpen
		}
		b.transition(HalfOpen) // this call is the trial
	case HalfOpen:
		return ErrOpen // a trial is already in flight
	}
	return nil
}

func (b *Breaker) record(err error) {
	b.mu.Lock()
	defer b.unlock()
	if b.state == Open {
		return // a straggler from before the circuit opened
	}
	if err == nil {
		b.failures = 0
		b.transition(Closed)
		return
	}
	b.failures++
	if b.state == HalfOpen || b.failures >= b.threshold {
		b.openedAt = b.now()
		b.transition(Open)
	}
}

// transition changes state and queues the notification. It requires b.mu.
func (b *Breaker) transition(to State) {
	if to == b.state {
		return
	}
	b.pending = append(b.pending, [2]State{b.state, to})
	b.state = to
}

// unlock releases b.mu and then reports the queued transitions.
func (b *Breaker) unlock() {
	pending, fn := b.pending, b.onChange
	b.pending = nil
	b.mu.Unlock()
	if fn != nil {
		for _, t := range pending {
			fn(t[0], t[1])
		}
	}
}
