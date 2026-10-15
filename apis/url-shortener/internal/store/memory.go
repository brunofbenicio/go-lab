package store

import (
	"context"
	"sync"
	"time"
)

// Memory is a Store that keeps links in a map guarded by a read/write mutex.
// Its zero value is not usable; create instances with NewMemory.
type Memory struct {
	mu    sync.RWMutex
	links map[string]Link
}

var _ Store = (*Memory)(nil)

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{links: make(map[string]Link)}
}

// Create implements Store.
func (m *Memory) Create(ctx context.Context, link Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.links[link.Code]; exists {
		return ErrConflict
	}
	m.links[link.Code] = link
	return nil
}

// Get implements Store.
func (m *Memory) Get(ctx context.Context, code string) (Link, error) {
	if err := ctx.Err(); err != nil {
		return Link{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	link, ok := m.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return link, nil
}

// RecordHit implements Store.
func (m *Memory) RecordHit(ctx context.Context, code string, at time.Time) (Link, error) {
	if err := ctx.Err(); err != nil {
		return Link{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	link, ok := m.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	link.Hits++
	link.LastAccessAt = at
	m.links[code] = link
	return link, nil
}

// Len returns the number of stored links. It is mostly useful in tests and
// diagnostics.
func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.links)
}
