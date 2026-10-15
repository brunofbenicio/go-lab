package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

func newUser(id, email string) User {
	return User{ID: id, Email: email, Name: "Test User", PasswordHash: "$hash", CreatedAt: t0}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct{ in, want string }{
		{"alice@example.com", "alice@example.com"},
		{"Alice@Example.COM", "alice@example.com"},
		{"  alice@example.com\t", "alice@example.com"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeEmail(tt.in); got != tt.want {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMemoryCreate(t *testing.T) {
	tests := []struct {
		name    string
		setup   []User
		create  User
		wantErr error
	}{
		{name: "first user", create: newUser("u1", "alice@example.com")},
		{name: "second user", setup: []User{newUser("u1", "alice@example.com")}, create: newUser("u2", "bob@example.com")},
		{name: "duplicate e-mail", setup: []User{newUser("u1", "alice@example.com")}, create: newUser("u2", "alice@example.com"), wantErr: ErrEmailTaken},
		{name: "duplicate e-mail differing in case", setup: []User{newUser("u1", "alice@example.com")}, create: newUser("u2", "ALICE@Example.com"), wantErr: ErrEmailTaken},
		{name: "duplicate e-mail differing in whitespace", setup: []User{newUser("u1", "alice@example.com")}, create: newUser("u2", " alice@example.com "), wantErr: ErrEmailTaken},
		{name: "duplicate id", setup: []User{newUser("u1", "alice@example.com")}, create: newUser("u1", "bob@example.com"), wantErr: ErrIDTaken},
		{name: "empty id", create: newUser("", "alice@example.com"), wantErr: ErrInvalidUser},
		{name: "empty e-mail", create: newUser("u1", ""), wantErr: ErrInvalidUser},
		{name: "blank e-mail", create: newUser("u1", "   "), wantErr: ErrInvalidUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := NewMemory()
			for _, u := range tt.setup {
				if err := m.Create(ctx, u); err != nil {
					t.Fatalf("setup Create(%q) failed: %v", u.ID, err)
				}
			}
			if err := m.Create(ctx, tt.create); !errors.Is(err, tt.wantErr) {
				t.Errorf("Create() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryFind(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if err := m.Create(ctx, newUser("u1", "Alice@Example.com")); err != nil {
		t.Fatal(err)
	}

	t.Run("by e-mail, any case", func(t *testing.T) {
		for _, email := range []string{"alice@example.com", "ALICE@EXAMPLE.COM", "  Alice@Example.com "} {
			got, err := m.FindByEmail(ctx, email)
			if err != nil {
				t.Fatalf("FindByEmail(%q) failed: %v", email, err)
			}
			if got.ID != "u1" || got.Email != "alice@example.com" {
				t.Errorf("FindByEmail(%q) = %+v", email, got)
			}
		}
	})

	t.Run("by id", func(t *testing.T) {
		got, err := m.FindByID(ctx, "u1")
		if err != nil {
			t.Fatal(err)
		}
		want := newUser("u1", "alice@example.com")
		if got != want {
			t.Errorf("FindByID() = %+v, want %+v", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := m.FindByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrNotFound) {
			t.Errorf("FindByEmail() error = %v, want ErrNotFound", err)
		}
		if _, err := m.FindByEmail(ctx, ""); !errors.Is(err, ErrNotFound) {
			t.Errorf("FindByEmail(\"\") error = %v, want ErrNotFound", err)
		}
		if _, err := m.FindByID(ctx, "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("FindByID() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMemoryFailedCreateLeavesNoTrace(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if err := m.Create(ctx, newUser("u1", "alice@example.com")); err != nil {
		t.Fatal(err)
	}

	// Same ID, different e-mail: rejected, and the e-mail must stay free.
	if err := m.Create(ctx, newUser("u1", "bob@example.com")); !errors.Is(err, ErrIDTaken) {
		t.Fatalf("Create() error = %v, want ErrIDTaken", err)
	}
	if _, err := m.FindByEmail(ctx, "bob@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rejected user is reachable by e-mail: %v", err)
	}
	if err := m.Create(ctx, newUser("u2", "bob@example.com")); err != nil {
		t.Errorf("the e-mail of a rejected user could not be reused: %v", err)
	}
}

func TestMemoryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewMemory()

	if err := m.Create(ctx, newUser("u1", "alice@example.com")); !errors.Is(err, context.Canceled) {
		t.Errorf("Create() error = %v, want context.Canceled", err)
	}
	if _, err := m.FindByEmail(ctx, "alice@example.com"); !errors.Is(err, context.Canceled) {
		t.Errorf("FindByEmail() error = %v, want context.Canceled", err)
	}
	if _, err := m.FindByID(ctx, "u1"); !errors.Is(err, context.Canceled) {
		t.Errorf("FindByID() error = %v, want context.Canceled", err)
	}
}

func TestMemoryConcurrentRegistrations(t *testing.T) {
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
			// Every worker fights for the same e-mail, spelled differently.
			email := "race@example.com"
			if i%2 == 1 {
				email = "RACE@example.com"
			}
			err := m.Create(ctx, newUser(fmt.Sprintf("u%d", i), email))
			switch {
			case err == nil:
				mu.Lock()
				successes++
				mu.Unlock()
			case !errors.Is(err, ErrEmailTaken):
				t.Errorf("Create() unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("%d registrations succeeded, want exactly 1", successes)
	}
}

func TestNewID(t *testing.T) {
	format := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := make(map[string]bool)
	for range 100 {
		id, err := NewID()
		if err != nil {
			t.Fatal(err)
		}
		if !format.MatchString(id) {
			t.Fatalf("NewID() = %q, want 32 lowercase hex characters", id)
		}
		if seen[id] {
			t.Fatalf("NewID() returned %q twice", id)
		}
		seen[id] = true
	}
}
