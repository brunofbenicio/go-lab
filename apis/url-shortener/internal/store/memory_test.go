package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

func newLink(code string) Link {
	return Link{Code: code, URL: "https://example.com/" + code, CreatedAt: t0}
}

func TestMemoryCreateAndGet(t *testing.T) {
	tests := []struct {
		name    string
		setup   []Link
		create  Link
		wantErr error
	}{
		{name: "new code", create: newLink("abc1234")},
		{name: "different code next to an existing one", setup: []Link{newLink("abc1234")}, create: newLink("xyz9876")},
		{name: "duplicate code", setup: []Link{newLink("abc1234")}, create: newLink("abc1234"), wantErr: ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := NewMemory()
			for _, l := range tt.setup {
				if err := m.Create(ctx, l); err != nil {
					t.Fatalf("setup Create(%q) failed: %v", l.Code, err)
				}
			}

			err := m.Create(ctx, tt.create)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			got, err := m.Get(ctx, tt.create.Code)
			if err != nil {
				t.Fatalf("Get() failed: %v", err)
			}
			if got != tt.create {
				t.Errorf("Get() = %+v, want %+v", got, tt.create)
			}
		})
	}
}

func TestMemoryCreateConflictKeepsOriginal(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	original := newLink("abc1234")
	if err := m.Create(ctx, original); err != nil {
		t.Fatal(err)
	}

	other := newLink("abc1234")
	other.URL = "https://example.org/other"
	if err := m.Create(ctx, other); !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() error = %v, want ErrConflict", err)
	}

	got, err := m.Get(ctx, "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != original.URL {
		t.Errorf("URL = %q, want the original %q", got.URL, original.URL)
	}
}

func TestMemoryGetNotFound(t *testing.T) {
	m := NewMemory()
	if _, err := m.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryRecordHit(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if err := m.Create(ctx, newLink("abc1234")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		code     string
		at       time.Time
		wantHits int64
		wantErr  error
	}{
		{name: "first hit", code: "abc1234", at: t0.Add(time.Minute), wantHits: 1},
		{name: "second hit", code: "abc1234", at: t0.Add(2 * time.Minute), wantHits: 2},
		{name: "unknown code", code: "nope", at: t0, wantErr: ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.RecordHit(ctx, tt.code, tt.at)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("RecordHit() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Hits != tt.wantHits {
				t.Errorf("Hits = %d, want %d", got.Hits, tt.wantHits)
			}
			if !got.LastAccessAt.Equal(tt.at) {
				t.Errorf("LastAccessAt = %v, want %v", got.LastAccessAt, tt.at)
			}

			stored, err := m.Get(ctx, tt.code)
			if err != nil {
				t.Fatal(err)
			}
			if stored != got {
				t.Errorf("stored link %+v differs from the returned one %+v", stored, got)
			}
		})
	}
}

func TestMemoryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewMemory()

	if err := m.Create(ctx, newLink("abc1234")); !errors.Is(err, context.Canceled) {
		t.Errorf("Create() error = %v, want context.Canceled", err)
	}
	if _, err := m.Get(ctx, "abc1234"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get() error = %v, want context.Canceled", err)
	}
	if _, err := m.RecordHit(ctx, "abc1234", t0); !errors.Is(err, context.Canceled) {
		t.Errorf("RecordHit() error = %v, want context.Canceled", err)
	}
	if m.Len() != 0 {
		t.Errorf("Len() = %d, want 0", m.Len())
	}
}

func TestMemoryConcurrentRecordHit(t *testing.T) {
	const (
		workers = 32
		perWork = 200
	)
	ctx := context.Background()
	m := NewMemory()
	if err := m.Create(ctx, newLink("abc1234")); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWork {
				if _, err := m.RecordHit(ctx, "abc1234", t0); err != nil {
					t.Errorf("RecordHit() failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, err := m.Get(ctx, "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(workers * perWork); got.Hits != want {
		t.Errorf("Hits = %d, want %d", got.Hits, want)
	}
}

func TestMemoryConcurrentCreateSameCode(t *testing.T) {
	const workers = 32
	ctx := context.Background()
	m := NewMemory()

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
	)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			link := newLink("samecode")
			link.URL = fmt.Sprintf("https://example.com/%d", i)
			if err := m.Create(ctx, link); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, ErrConflict) {
				t.Errorf("Create() unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("%d creations succeeded, want exactly 1", successes)
	}
	if m.Len() != 1 {
		t.Errorf("Len() = %d, want 1", m.Len())
	}
}

func TestLinkExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{name: "no expiry never expires", now: t0.Add(1000 * time.Hour), want: false},
		{name: "before expiry", expiresAt: t0.Add(time.Hour), now: t0, want: false},
		{name: "one nanosecond before expiry", expiresAt: t0.Add(time.Hour), now: t0.Add(time.Hour - 1), want: false},
		{name: "exactly at expiry", expiresAt: t0.Add(time.Hour), now: t0.Add(time.Hour), want: true},
		{name: "after expiry", expiresAt: t0.Add(time.Hour), now: t0.Add(2 * time.Hour), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := Link{Code: "abc1234", ExpiresAt: tt.expiresAt}
			if got := l.Expired(tt.now); got != tt.want {
				t.Errorf("Expired(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}
