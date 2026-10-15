// Package workerpool offers two ways to bound concurrency: Pool, a fixed set
// of long-lived workers that consume submitted tasks, and Map, a one-shot
// parallel transformation of a slice that preserves order and stops at the
// first error.
package workerpool

import (
	"context"
	"errors"
	"sync"
)

// ErrClosed is returned by Submit after Close.
var ErrClosed = errors.New("workerpool: pool is closed")

// Pool runs submitted tasks on a fixed number of goroutines. A task that
// panics crashes the program, like any other goroutine.
type Pool struct {
	mu     sync.RWMutex
	closed bool
	tasks  chan func()
	wg     sync.WaitGroup
}

// New starts a pool with the given number of workers and a queue that holds
// up to queueSize pending tasks. workers < 1 becomes 1; queueSize < 0 becomes
// 0 (Submit then hands tasks directly to idle workers).
func New(workers, queueSize int) *Pool {
	workers, queueSize = max(workers, 1), max(queueSize, 0)
	p := &Pool{tasks: make(chan func(), queueSize)}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer p.wg.Done()
			for task := range p.tasks {
				task()
			}
		}()
	}
	return p
}

// Submit queues task for execution. It blocks while the queue is full, until
// ctx is done (returning ctx's error) or the pool is closed.
func (p *Pool) Submit(ctx context.Context, task func()) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case p.tasks <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops accepting tasks, waits for the queued and running ones to
// finish and stops the workers. It is safe to call more than once.
func (p *Pool) Close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.tasks)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// Map applies fn to every element of in using up to workers goroutines and
// returns the results in input order. The first error cancels the context
// passed to fn, stops dispatching new elements and is returned; results of
// elements that did not complete hold the zero value. If ctx ends first, its
// error is returned.
func Map[T, R any](ctx context.Context, workers int, in []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	out := make([]R, len(in))
	indexes := make(chan int)
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	for w := 0; w < max(workers, 1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				r, err := fn(ctx, in[i])
				if err != nil {
					once.Do(func() { firstErr = err; cancel() })
					continue
				}
				out[i] = r
			}
		}()
	}

	aborted := false
dispatch:
	for i := range in {
		select {
		case indexes <- i:
		case <-ctx.Done():
			aborted = true
			break dispatch
		}
	}
	close(indexes)
	wg.Wait()

	if firstErr != nil {
		return out, firstErr
	}
	if aborted {
		return out, parent.Err()
	}
	return out, nil
}
