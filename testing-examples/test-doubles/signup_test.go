package signup

import (
	"context"
	"errors"
	"testing"
)

// Compile-time checks that the doubles really implement the interfaces.
var (
	_ UserStore = (*stubStore)(nil)
	_ UserStore = (*memStore)(nil)
	_ Mailer    = (*spyMailer)(nil)
)

// stubStore is a STUB: it returns canned answers and has no logic.
type stubStore struct {
	existing *User
	err      error
}

func (s stubStore) ByEmail(context.Context, string) (*User, error) { return s.existing, s.err }
func (s stubStore) Save(context.Context, User) error               { return nil }

// memStore is a FAKE: a real, working implementation, just simpler than the
// production one (a map instead of a database).
type memStore struct{ users map[string]User }

func newMemStore() *memStore { return &memStore{users: map[string]User{}} }

func (m *memStore) ByEmail(_ context.Context, email string) (*User, error) {
	if u, ok := m.users[email]; ok {
		return &u, nil
	}
	return nil, nil
}

func (m *memStore) Save(_ context.Context, u User) error {
	m.users[u.Email] = u
	return nil
}

// spyMailer is a SPY / hand-written MOCK: it records every call so the test
// can verify the interaction afterwards, and can be told to fail.
type spyMailer struct {
	sent []sentMail
	err  error
}

type sentMail struct{ to, subject, body string }

func (s *spyMailer) Send(_ context.Context, to, subject, body string) error {
	s.sent = append(s.sent, sentMail{to, subject, body})
	return s.err
}

func TestRegisterSuccess(t *testing.T) {
	store, mailer := newMemStore(), &spyMailer{}
	svc := NewService(store, mailer)

	u, err := svc.Register(t.Context(), "ana@example.com", "Ana")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// State check against the fake...
	if got := store.users["ana@example.com"]; got != u {
		t.Errorf("stored user = %+v, want %+v", got, u)
	}
	// ...and interaction check against the spy.
	want := sentMail{"ana@example.com", "Welcome", "Hello, Ana!"}
	if len(mailer.sent) != 1 || mailer.sent[0] != want {
		t.Errorf("sent mails = %+v, want exactly [%+v]", mailer.sent, want)
	}
}

func TestRegisterEmailTaken(t *testing.T) {
	// The stub is enough here: we only need ByEmail to report a hit.
	store := stubStore{existing: &User{Email: "ana@example.com"}}
	mailer := &spyMailer{}

	_, err := NewService(store, mailer).Register(t.Context(), "ana@example.com", "Ana")

	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("error = %v, want ErrEmailTaken", err)
	}
	if len(mailer.sent) != 0 {
		t.Errorf("sent %d mails, want none for a duplicate", len(mailer.sent))
	}
}

func TestRegisterInvalidEmailTouchesNothing(t *testing.T) {
	store, mailer := newMemStore(), &spyMailer{}

	_, err := NewService(store, mailer).Register(t.Context(), "not-an-email", "Ana")

	if !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("error = %v, want ErrInvalidEmail", err)
	}
	if len(store.users) != 0 || len(mailer.sent) != 0 {
		t.Errorf("store=%v mails=%v, want both untouched", store.users, mailer.sent)
	}
}

func TestRegisterStoreFailure(t *testing.T) {
	boom := errors.New("connection reset")
	svc := NewService(stubStore{err: boom}, &spyMailer{})

	_, err := svc.Register(t.Context(), "ana@example.com", "Ana")

	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want it to wrap the store error", err)
	}
}

func TestRegisterMailFailureKeepsUser(t *testing.T) {
	boom := errors.New("smtp down")
	store := newMemStore()
	svc := NewService(store, &spyMailer{err: boom})

	u, err := svc.Register(t.Context(), "ana@example.com", "Ana")

	if !errors.Is(err, ErrWelcomeMail) || !errors.Is(err, boom) {
		t.Errorf("error = %v, want ErrWelcomeMail wrapping the SMTP error", err)
	}
	if u.Email == "" {
		t.Error("returned user is empty, want the saved user")
	}
	if _, ok := store.users["ana@example.com"]; !ok {
		t.Error("user was not saved before the mail failure")
	}
}
