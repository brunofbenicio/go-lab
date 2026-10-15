// Package signup registers new users. The Service depends on two small
// interfaces, UserStore and Mailer, which lets tests replace the database and
// the mail server with hand-written doubles.
package signup

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Errors returned by Register.
var (
	ErrInvalidEmail = errors.New("signup: invalid email")
	ErrEmailTaken   = errors.New("signup: email already registered")
	ErrWelcomeMail  = errors.New("signup: welcome mail failed")
)

// User is a registered account.
type User struct {
	Email string
	Name  string
}

// UserStore persists users. ByEmail returns (nil, nil) when nobody matches.
type UserStore interface {
	ByEmail(ctx context.Context, email string) (*User, error)
	Save(ctx context.Context, u User) error
}

// Mailer sends e-mail.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Service implements the registration use case.
type Service struct {
	store  UserStore
	mailer Mailer
}

// NewService wires a Service to its dependencies.
func NewService(store UserStore, mailer Mailer) *Service {
	return &Service{store: store, mailer: mailer}
}

// Register validates and stores a new user, then sends a welcome mail. A mail
// failure is not fatal: the user is already saved, so Register returns the
// user together with an error wrapping ErrWelcomeMail.
func (s *Service) Register(ctx context.Context, email, name string) (User, error) {
	if !strings.Contains(email, "@") {
		return User{}, fmt.Errorf("%w: %q", ErrInvalidEmail, email)
	}
	existing, err := s.store.ByEmail(ctx, email)
	if err != nil {
		return User{}, fmt.Errorf("look up %q: %w", email, err)
	}
	if existing != nil {
		return User{}, ErrEmailTaken
	}
	u := User{Email: email, Name: name}
	if err := s.store.Save(ctx, u); err != nil {
		return User{}, fmt.Errorf("save %q: %w", email, err)
	}
	if err := s.mailer.Send(ctx, email, "Welcome", "Hello, "+name+"!"); err != nil {
		return u, fmt.Errorf("%w: %w", ErrWelcomeMail, err)
	}
	return u, nil
}
