package todo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeClock returns a clock that advances one minute per call.
func fakeClock() func() time.Time {
	current := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var mu sync.Mutex
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		defer func() { current = current.Add(time.Minute) }()
		return current
	}
}

func TestMemoryStoreCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStoreWithClock(fakeClock())

	first, err := s.Create(ctx, Input{Title: "first"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, _ := s.Create(ctx, Input{Title: "second", Notes: "n", Done: true})

	if first.ID != 1 || second.ID != 2 {
		t.Errorf("IDs = %d, %d, want 1, 2", first.ID, second.ID)
	}
	if !first.CreatedAt.Equal(first.UpdatedAt) {
		t.Errorf("new todo must have CreatedAt == UpdatedAt, got %v and %v", first.CreatedAt, first.UpdatedAt)
	}
	if !second.CreatedAt.After(first.CreatedAt) {
		t.Errorf("second CreatedAt %v should be after first %v", second.CreatedAt, first.CreatedAt)
	}

	got, err := s.Get(ctx, 2)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Title != "second" || got.Notes != "n" || !got.Done {
		t.Errorf("Get() = %+v, want the second todo", got)
	}
}

func TestMemoryStoreNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	if _, err := s.Get(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if _, err := s.Replace(ctx, 99, Input{Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Replace() error = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreListIsOrderedByID(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	for _, title := range []string{"c", "a", "b"} {
		if _, err := s.Create(ctx, Input{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Delete(ctx, 2); err != nil {
		t.Fatal(err)
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("List() = %+v, want IDs [1 3]", got)
	}
}

func TestMemoryStoreReplace(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStoreWithClock(fakeClock())
	created, _ := s.Create(ctx, Input{Title: "old", Notes: "keep?"})
	due := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	updated, err := s.Replace(ctx, created.ID, Input{Title: "new", Done: true, DueDate: &due})
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	if updated.Title != "new" || !updated.Done || updated.Notes != "" {
		t.Errorf("Replace() = %+v, want title new, done, empty notes", updated)
	}
	if updated.DueDate == nil || !updated.DueDate.Equal(due) {
		t.Errorf("DueDate = %v, want %v", updated.DueDate, due)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt changed from %v to %v", created.CreatedAt, updated.CreatedAt)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt %v should be after %v", updated.UpdatedAt, created.UpdatedAt)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	created, _ := s.Create(ctx, Input{Title: "gone soon"})

	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after Delete error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreDoesNotAliasDueDate(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	due := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	created, _ := s.Create(ctx, Input{Title: "x", DueDate: &due})

	// Mutating the caller's value or the returned value must not leak into the store.
	due = due.Add(24 * time.Hour)
	*created.DueDate = created.DueDate.Add(48 * time.Hour)

	got, _ := s.Get(ctx, created.ID)
	want := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	if !got.DueDate.Equal(want) {
		t.Errorf("stored DueDate = %v, want %v", got.DueDate, want)
	}
}

func TestMemoryStoreConcurrentCreates(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	const n = 100

	var wg sync.WaitGroup
	ids := make(chan int64, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			created, err := s.Create(ctx, Input{Title: "parallel"})
			if err != nil {
				t.Errorf("Create() error = %v", err)
				return
			}
			ids <- created.ID
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int64]bool, n)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID %d", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("got %d distinct IDs, want %d", len(seen), n)
	}
}
