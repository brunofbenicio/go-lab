// Package singleflight suppresses duplicate concurrent work: while a call for
// a key is in flight, other callers with the same key wait for it and share
// its result instead of repeating it. It protects a backend from a "cache
// stampede" when a popular entry expires.
package singleflight

import (
	"errors"
	"sync"
)

// ErrPanic is returned to callers that were waiting on a call whose function
// panicked. The panic itself propagates in the goroutine that ran it.
var ErrPanic = errors.New("singleflight: call panicked")

type call[V any] struct {
	wg   sync.WaitGroup
	val  V
	err  error
	dups int // callers that joined an existing call
}

// Group deduplicates calls by key. The zero value is ready to use.
type Group[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*call[V]
}

// Do runs fn for key and returns its result. If a call for the same key is
// already running, Do waits for it and returns the same result. shared
// reports whether the result was handed to more than one caller; it is true
// for the caller that ran fn as well, when others joined it. Once the call
// finishes the key is forgotten, so the next Do runs fn again.
func (g *Group[K, V]) Do(key K, fn func() (V, error)) (v V, shared bool, err error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*call[V])
	}
	if c, ok := g.calls[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, true, c.err
	}
	c := new(call[V])
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	finished := false
	defer func() {
		if !finished {
			c.err = ErrPanic
		}
		g.mu.Lock()
		delete(g.calls, key)
		shared = c.dups > 0
		g.mu.Unlock()
		c.wg.Done()
	}()
	c.val, c.err = fn()
	finished = true
	return c.val, false, c.err
}
