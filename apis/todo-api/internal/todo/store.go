package todo

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when a Todo does not exist.
var ErrNotFound = errors.New("todo: not found")

// Store is the persistence contract used by the HTTP layer. Implementations
// must be safe for concurrent use and assume the input is already validated.
type Store interface {
	// Create stores a new Todo and returns it with its generated ID.
	Create(ctx context.Context, in Input) (Todo, error)
	// Get returns the Todo with the given ID or ErrNotFound.
	Get(ctx context.Context, id int64) (Todo, error)
	// List returns the todos matching opts, ordered by ID.
	List(ctx context.Context, opts ListOptions) (ListResult, error)
	// Replace overwrites the client-controlled fields of a Todo.
	Replace(ctx context.Context, id int64, in Input) (Todo, error)
	// Patch applies a partial update. The merged result is validated, so it
	// can return a *ValidationError as well as ErrNotFound.
	Patch(ctx context.Context, id int64, p Patch) (Todo, error)
	// Delete removes a Todo or returns ErrNotFound.
	Delete(ctx context.Context, id int64) error
}

// MemoryStore is a Store that keeps everything in memory. It is meant for
// tests, demos and local development.
type MemoryStore struct {
	mu     sync.RWMutex
	items  map[int64]Todo
	nextID int64
	now    func() time.Time
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty store that stamps records with the wall clock.
func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithClock(time.Now)
}

// NewMemoryStoreWithClock returns an empty store using now as its clock, which
// makes timestamps deterministic in tests.
func NewMemoryStoreWithClock(now func() time.Time) *MemoryStore {
	return &MemoryStore{items: make(map[int64]Todo), nextID: 1, now: now}
}

// Create implements Store.
func (s *MemoryStore) Create(_ context.Context, in Input) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stamp := s.now().UTC()
	t := Todo{
		ID:        s.nextID,
		Title:     in.Title,
		Notes:     in.Notes,
		Done:      in.Done,
		DueDate:   copyTime(in.DueDate),
		CreatedAt: stamp,
		UpdatedAt: stamp,
	}
	s.items[t.ID] = t
	s.nextID++
	return clone(t), nil
}

// Get implements Store.
func (s *MemoryStore) Get(_ context.Context, id int64) (Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	return clone(t), nil
}

// List implements Store.
func (s *MemoryStore) List(_ context.Context, opts ListOptions) (ListResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := strings.ToLower(strings.TrimSpace(opts.Query))
	matched := make([]Todo, 0, len(s.items))
	for _, t := range s.items {
		if opts.Done != nil && t.Done != *opts.Done {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(t.Title), query) &&
			!strings.Contains(strings.ToLower(t.Notes), query) {
			continue
		}
		matched = append(matched, clone(t))
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })

	total := len(matched)
	start := min(max(opts.Offset, 0), total)
	end := total
	if opts.Limit > 0 {
		end = min(start+opts.Limit, total)
	}
	return ListResult{Items: matched[start:end], Total: total}, nil
}

// Replace implements Store.
func (s *MemoryStore) Replace(_ context.Context, id int64, in Input) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	t.Title = in.Title
	t.Notes = in.Notes
	t.Done = in.Done
	t.DueDate = copyTime(in.DueDate)
	t.UpdatedAt = s.now().UTC()
	s.items[id] = t
	return clone(t), nil
}

// Patch implements Store.
func (s *MemoryStore) Patch(_ context.Context, id int64, p Patch) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	if p.IsEmpty() {
		return clone(t), nil
	}

	in := p.Apply(t).Normalized()
	if err := in.Validate(); err != nil {
		return Todo{}, err
	}
	t.Title = in.Title
	t.Notes = in.Notes
	t.Done = in.Done
	t.DueDate = copyTime(in.DueDate)
	t.UpdatedAt = s.now().UTC()
	s.items[id] = t
	return clone(t), nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

// clone detaches the pointer fields so callers cannot mutate stored state.
func clone(t Todo) Todo {
	t.DueDate = copyTime(t.DueDate)
	return t
}

func copyTime(p *time.Time) *time.Time {
	if p == nil {
		return nil
	}
	c := *p
	return &c
}
