package singleflight

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// waiters reports how many callers have joined the in-flight call for key.
func (g *Group[K, V]) waiters(key K) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if c, ok := g.calls[key]; ok {
		return c.dups
	}
	return -1
}

type result struct {
	v      int
	shared bool
	err    error
}

func TestDoDeduplicatesConcurrentCalls(t *testing.T) {
	const callers = 6
	var g Group[string, int]
	var runs atomic.Int32
	release := make(chan struct{})
	results := make(chan result, callers)

	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, shared, err := g.Do("k", func() (int, error) {
				runs.Add(1)
				<-release
				return 42, nil
			})
			results <- result{v, shared, err}
		}()
	}
	// Hold the leader until every other caller has joined its call.
	for g.waiters("k") != callers-1 {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()
	close(results)

	for r := range results {
		if r.v != 42 || r.err != nil {
			t.Errorf("got (%d, %v), want (42, nil)", r.v, r.err)
		}
		if !r.shared {
			t.Error("every caller, leader included, should see shared=true when the result was handed to several callers")
		}
	}
	if runs.Load() != 1 {
		t.Errorf("fn ran %d times, want 1", runs.Load())
	}
}

func TestDoRunsAgainAfterCompletion(t *testing.T) {
	var g Group[string, int]
	runs := 0
	fn := func() (int, error) { runs++; return runs, nil }

	for want := 1; want <= 3; want++ {
		v, shared, err := g.Do("k", fn)
		if v != want || shared || err != nil {
			t.Fatalf("call %d: got (%d, %v, %v), want (%d, false, nil)", want, v, shared, err, want)
		}
	}
}

func TestDoDistinctKeysDoNotShare(t *testing.T) {
	var g Group[int, string]
	a, sharedA, _ := g.Do(1, func() (string, error) { return "one", nil })
	b, sharedB, _ := g.Do(2, func() (string, error) { return "two", nil })
	if a != "one" || b != "two" || sharedA || sharedB {
		t.Errorf("got (%q, %v) and (%q, %v)", a, sharedA, b, sharedB)
	}
}

func TestDoPropagatesErrors(t *testing.T) {
	var g Group[string, int]
	boom := errors.New("boom")
	if _, _, err := g.Do("k", func() (int, error) { return 0, boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestDoPanicReleasesKeyAndWaiters(t *testing.T) {
	var g Group[string, int]
	release := make(chan struct{})
	waiter := make(chan result, 1)

	leaderDone := make(chan any, 1)
	go func() {
		defer func() { leaderDone <- recover() }()
		_, _, _ = g.Do("k", func() (int, error) {
			<-release
			panic("kaboom")
		})
	}()
	for g.waiters("k") < 0 {
		runtime.Gosched()
	}
	go func() {
		v, shared, err := g.Do("k", func() (int, error) { return 1, nil })
		waiter <- result{v, shared, err}
	}()
	for g.waiters("k") < 1 {
		runtime.Gosched()
	}
	close(release)

	if r := <-leaderDone; r != "kaboom" {
		t.Errorf("leader recovered %v, want the original panic", r)
	}
	if r := <-waiter; !errors.Is(r.err, ErrPanic) || !r.shared {
		t.Errorf("waiter got %+v, want ErrPanic and shared", r)
	}
	if v, _, err := g.Do("k", func() (int, error) { return 7, nil }); v != 7 || err != nil {
		t.Errorf("key was not released after the panic: (%d, %v)", v, err)
	}
}
