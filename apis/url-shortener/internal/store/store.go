// Package store defines the persistence contract of the URL shortener and
// ships a thread-safe in-memory implementation.
//
// The interface is deliberately small so that a database-backed
// implementation can replace the in-memory one without touching the service
// or the HTTP layers.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound is returned when no link exists for the requested code.
	ErrNotFound = errors.New("store: link not found")
	// ErrConflict is returned by Create when the code is already taken.
	ErrConflict = errors.New("store: code already exists")
)

// Link is a shortened URL together with its usage counters.
type Link struct {
	// Code is the short identifier, unique within a Store.
	Code string
	// URL is the original destination.
	URL string
	// CreatedAt is the moment the link was created.
	CreatedAt time.Time
	// ExpiresAt is the moment the link stops redirecting. The zero value means
	// the link never expires.
	ExpiresAt time.Time
	// Hits counts how many times the link has been followed.
	Hits int64
	// LastAccessAt is the moment of the most recent hit. It is the zero value
	// when the link has never been followed.
	LastAccessAt time.Time
}

// Expired reports whether the link is no longer valid at the instant now.
// A link is expired from ExpiresAt onwards; links without ExpiresAt never
// expire.
func (l Link) Expired(now time.Time) bool {
	return !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt)
}

// Store persists links. Implementations must be safe for concurrent use.
type Store interface {
	// Create saves a new link. It returns ErrConflict if a link with the same
	// code already exists; the existing link is left untouched.
	Create(ctx context.Context, link Link) error

	// Get returns the link stored under code, or ErrNotFound.
	Get(ctx context.Context, code string) (Link, error)

	// RecordHit atomically increments the hit counter of the link, sets its
	// LastAccessAt to at, and returns the updated link. It returns ErrNotFound
	// if the code does not exist.
	RecordHit(ctx context.Context, code string, at time.Time) (Link, error)
}
