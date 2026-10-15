package metrics

import (
	"slices"
	"sync"
	"testing"
)

// The race detector only reports races that actually happen during the run, so
// a test has to make goroutines touch the shared state at the same time. These
// tests start many goroutines doing many operations each, then wait for all of
// them with a WaitGroup (no sleeps, no leaked goroutines).
//
//	go test -race ./testing-examples/race-detector/
const (
	workers = 16
	perWork = 500
)

// parallel runs fn in `workers` goroutines and waits for all of them.
func parallel(fn func(worker int)) {
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(w)
		}()
	}
	wg.Wait()
}

func TestCounterConcurrentIncrements(t *testing.T) {
	var c Counter

	parallel(func(int) {
		for range perWork {
			c.Inc()
		}
	})

	// Besides being race-free, the total must be exact: with an unsynchronized
	// `n++`, increments would be lost and this number would come out lower.
	if got, want := c.Value(), int64(workers*perWork); got != want {
		t.Errorf("Value() = %d, want %d", got, want)
	}
}

func TestRegistryConcurrentCreationReturnsSameCounter(t *testing.T) {
	r := NewRegistry()
	got := make([]*Counter, workers)

	parallel(func(w int) { got[w] = r.Counter("requests") }) // each goroutine writes its own slot

	for i, c := range got {
		if c != got[0] {
			t.Fatalf("worker %d got a different counter: two goroutines created %q at once", i, "requests")
		}
	}
	if names := r.Names(); !slices.Equal(names, []string{"requests"}) {
		t.Errorf("Names() = %v, want [requests]", names)
	}
}

// Writers and readers at the same time: Snapshot must not race with updates.
func TestRegistryReadersAndWriters(t *testing.T) {
	r := NewRegistry()

	parallel(func(w int) {
		if w%4 == 0 {
			for range perWork {
				_ = r.Snapshot() // reader
				_ = r.Names()
			}
			return
		}
		name := []string{"a", "b", "c"}[w%3] // writers share three counters
		for range perWork {
			r.Counter(name).Inc()
		}
	})

	var total int64
	for _, v := range r.Snapshot() {
		total += v
	}
	writers := workers - workers/4
	if want := int64(writers * perWork); total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	r := NewRegistry()
	r.Counter("hits").Add(5)

	snap := r.Snapshot()
	snap["hits"] = 999 // must not affect the registry
	r.Counter("hits").Inc()

	if got := r.Snapshot()["hits"]; got != 6 {
		t.Errorf("hits = %d, want 6", got)
	}
	if snap["hits"] != 999 {
		t.Error("later updates must not alter an earlier snapshot")
	}
}
