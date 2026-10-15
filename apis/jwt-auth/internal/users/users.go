// Package users defines the user model of the authentication API and a
// thread-safe in-memory store for it.
//
// The Store interface is small on purpose, so a database-backed implementation
// can replace the in-memory one without touching the rest of the application.
package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotFound is returned when no user matches the lookup.
	ErrNotFound = errors.New("users: user not found")
	// ErrEmailTaken is returned by Create when the e-mail is already registered.
	ErrEmailTaken = errors.New("users: email already registered")
	// ErrIDTaken is returned by Create when the ID is already in use.
	ErrIDTaken = errors.New("users: id already in use")
	// ErrInvalidUser is returned by Create when the ID or the e-mail is empty.
	ErrInvalidUser = errors.New("users: id and email are required")
)

// User is an account of the API.
type User struct {
	// ID is the immutable identifier of the user.
	ID string
	// Email is the login of the user, stored normalized (see NormalizeEmail).
	Email string
	// Name is an optional display name.
	Name string
	// PasswordHash is the encoded password hash. It must never leave the server.
	PasswordHash string
	// CreatedAt is the moment the account was created.
	CreatedAt time.Time
}

// Store persists users. Implementations must be safe for concurrent use.
type Store interface {
	// Create saves a new user. It returns ErrEmailTaken when another user has
	// the same e-mail (compared after normalization), ErrIDTaken when the ID
	// exists and ErrInvalidUser when the ID or the e-mail is empty.
	Create(ctx context.Context, user User) error

	// FindByEmail returns the user with the given e-mail, compared after
	// normalization, or ErrNotFound.
	FindByEmail(ctx context.Context, email string) (User, error)

	// FindByID returns the user with the given ID, or ErrNotFound.
	FindByID(ctx context.Context, id string) (User, error)
}

// NormalizeEmail returns the canonical form of an e-mail used for uniqueness:
// surrounding whitespace removed and letters lowercased.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NewID returns a random 128-bit identifier as 32 lowercase hex characters.
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("users: generate id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// Memory is a Store that keeps users in maps guarded by a mutex. Create
// instances with NewMemory.
type Memory struct {
	mu      sync.RWMutex
	byID    map[string]User
	byEmail map[string]string // normalized e-mail -> ID
}

var _ Store = (*Memory)(nil)

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		byID:    make(map[string]User),
		byEmail: make(map[string]string),
	}
}

// Create implements Store.
func (m *Memory) Create(ctx context.Context, user User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	user.Email = NormalizeEmail(user.Email)
	if user.ID == "" || user.Email == "" {
		return ErrInvalidUser
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byEmail[user.Email]; exists {
		return ErrEmailTaken
	}
	if _, exists := m.byID[user.ID]; exists {
		return ErrIDTaken
	}
	m.byID[user.ID] = user
	m.byEmail[user.Email] = user.ID
	return nil
}

// FindByEmail implements Store.
func (m *Memory) FindByEmail(ctx context.Context, email string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byEmail[NormalizeEmail(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return m.byID[id], nil
}

// FindByID implements Store.
func (m *Memory) FindByID(ctx context.Context, id string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}
