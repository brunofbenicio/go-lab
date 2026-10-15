// Package metrics is a small in-process metrics registry that many goroutines
// update at once. Its synchronization is what the tests (run with -race) verify.
package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing number. The zero value is ready to use
// and it is safe for concurrent use: a plain `n++` from several goroutines would
// be a data race, so the value lives in an atomic.
type Counter struct{ n atomic.Int64 }

// Inc adds one.
func (c *Counter) Inc() { c.n.Add(1) }

// Add adds delta.
func (c *Counter) Add(delta int64) { c.n.Add(delta) }

// Value returns the current count.
func (c *Counter) Value() int64 { return c.n.Load() }

// Registry holds named counters, creating them on first use.
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{counters: make(map[string]*Counter)}
}

// Counter returns the counter called name, creating it if needed. The map is
// guarded by a mutex: Go maps are not safe for concurrent read/write, and the
// runtime may even abort the program ("concurrent map writes").
func (r *Registry) Counter(name string) *Counter {
	r.mu.RLock()
	c, ok := r.counters[name]
	r.mu.RUnlock()
	if ok {
		return c
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok = r.counters[name]; ok { // re-check: another goroutine may have won
		return c
	}
	c = new(Counter)
	r.counters[name] = c
	return c
}

// Snapshot returns a copy of every counter's current value. The copy can be
// used freely by the caller without any locking.
func (r *Registry) Snapshot() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]int64, len(r.counters))
	for name, c := range r.counters {
		out[name] = c.Value()
	}
	return out
}

// Names returns the counter names in alphabetical order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.counters))
	for name := range r.counters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
