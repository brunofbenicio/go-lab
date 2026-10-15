// Package shortener implements the business rules of the URL shortener:
// validating destinations, generating unique short codes, handling code
// collisions, expiring links and counting hits.
//
// The service depends only on a store.Store, a clock and a source of random
// bytes. The last two can be replaced, which keeps the behavior fully
// deterministic in tests.
package shortener

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/base62"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/store"
)

// Limits and defaults of the service.
const (
	// DefaultCodeLength is the length of random codes unless overridden.
	DefaultCodeLength = 7
	// MinCodeLength is the smallest allowed code length. Five characters keep
	// random codes from ever colliding with reserved single-segment routes
	// such as "docs".
	MinCodeLength = 5
	// MaxCodeLength is the largest allowed code length. Ten base62 digits are
	// the most that always fit in a uint64.
	MaxCodeLength = 10
	// DefaultMaxAttempts is how many codes are tried before giving up on a
	// collision streak.
	DefaultMaxAttempts = 5
	// DefaultMaxTTL is the longest lifetime a link may be given.
	DefaultMaxTTL = 365 * 24 * time.Hour
	// MaxURLLength is the longest accepted destination URL, in bytes.
	MaxURLLength = 2048
)

var (
	// ErrInvalidURL is returned (wrapped with a reason) when the destination
	// URL is rejected.
	ErrInvalidURL = errors.New("invalid URL")
	// ErrInvalidTTL is returned (wrapped with a reason) when the requested
	// lifetime is rejected.
	ErrInvalidTTL = errors.New("invalid TTL")
	// ErrNotFound is returned when the code does not exist.
	ErrNotFound = errors.New("link not found")
	// ErrExpired is returned when the link exists but its lifetime is over.
	ErrExpired = errors.New("link expired")
	// ErrCodeGeneration is returned when every attempt at generating a unique
	// code collided with an existing link.
	ErrCodeGeneration = errors.New("could not generate a unique code")
)

// ShortenInput is the request to create a short link.
type ShortenInput struct {
	// URL is the destination. Only absolute http and https URLs are accepted.
	URL string
	// TTL is how long the link stays valid. Zero means it never expires.
	TTL time.Duration
}

// Stats describes a link and its usage at the time it was read.
type Stats struct {
	store.Link
	// Expired reports whether the link is already past its ExpiresAt.
	Expired bool
}

// Service creates, resolves and reports on short links.
type Service struct {
	store       store.Store
	now         func() time.Time
	random      io.Reader
	codeLength  int
	maxAttempts int
	maxTTL      time.Duration
	// codeSpace is 62^codeLength, the number of distinct codes.
	codeSpace uint64
}

// Option customizes a Service.
type Option func(*Service)

// WithClock replaces the time source. A nil function is ignored.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithRandom replaces the source of random bytes used to generate codes. By
// default crypto/rand is used. A nil reader is ignored.
func WithRandom(r io.Reader) Option {
	return func(s *Service) {
		if r != nil {
			s.random = r
		}
	}
}

// WithCodeLength sets the length of the random codes. New rejects values
// outside [MinCodeLength, MaxCodeLength].
func WithCodeLength(n int) Option {
	return func(s *Service) { s.codeLength = n }
}

// WithMaxAttempts sets how many candidate codes are tried when collisions
// happen. New rejects values below one.
func WithMaxAttempts(n int) Option {
	return func(s *Service) { s.maxAttempts = n }
}

// WithMaxTTL sets the longest lifetime a link may be given. New rejects
// non-positive values.
func WithMaxTTL(d time.Duration) Option {
	return func(s *Service) { s.maxTTL = d }
}

// New builds a Service on top of st. It returns an error when an option holds
// an out-of-range value.
func New(st store.Store, opts ...Option) (*Service, error) {
	if st == nil {
		return nil, errors.New("shortener: store must not be nil")
	}
	s := &Service{
		store:       st,
		now:         time.Now,
		random:      cryptorand.Reader,
		codeLength:  DefaultCodeLength,
		maxAttempts: DefaultMaxAttempts,
		maxTTL:      DefaultMaxTTL,
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.codeLength < MinCodeLength || s.codeLength > MaxCodeLength {
		return nil, fmt.Errorf("shortener: code length must be between %d and %d, got %d",
			MinCodeLength, MaxCodeLength, s.codeLength)
	}
	if s.maxAttempts < 1 {
		return nil, fmt.Errorf("shortener: max attempts must be at least 1, got %d", s.maxAttempts)
	}
	if s.maxTTL <= 0 {
		return nil, fmt.Errorf("shortener: max TTL must be positive, got %s", s.maxTTL)
	}

	s.codeSpace = 1
	for range s.codeLength {
		s.codeSpace *= 62
	}
	return s, nil
}

// Shorten validates the request and stores a new link under a freshly
// random code. When a code is already taken, another one is
// drawn, up to the configured number of attempts.
//
// It returns ErrInvalidURL or ErrInvalidTTL (wrapped with a reason) for bad
// input and ErrCodeGeneration if every attempt collided.
func (s *Service) Shorten(ctx context.Context, in ShortenInput) (store.Link, error) {
	target, err := validateURL(in.URL)
	if err != nil {
		return store.Link{}, err
	}
	if err := s.validateTTL(in.TTL); err != nil {
		return store.Link{}, err
	}

	now := s.now().UTC()
	link := store.Link{URL: target, CreatedAt: now}
	if in.TTL > 0 {
		link.ExpiresAt = now.Add(in.TTL)
	}

	for range s.maxAttempts {
		code, err := s.newCode()
		if err != nil {
			return store.Link{}, fmt.Errorf("generate code: %w", err)
		}
		link.Code = code

		err = s.store.Create(ctx, link)
		switch {
		case err == nil:
			return link, nil
		case errors.Is(err, store.ErrConflict):
			continue
		default:
			return store.Link{}, err
		}
	}
	return store.Link{}, ErrCodeGeneration
}

// Resolve looks up the destination of code and records the access. Expired
// links are not counted as hits.
//
// It returns ErrNotFound for unknown codes and ErrExpired for links past their
// lifetime.
func (s *Service) Resolve(ctx context.Context, code string) (store.Link, error) {
	link, err := s.store.Get(ctx, code)
	if err != nil {
		return store.Link{}, translate(err)
	}
	now := s.now()
	if link.Expired(now) {
		return store.Link{}, ErrExpired
	}
	link, err = s.store.RecordHit(ctx, code, now.UTC())
	if err != nil {
		return store.Link{}, translate(err)
	}
	return link, nil
}

// Stats returns the link and its counters without recording a hit. Expired
// links still have stats, flagged with Expired.
func (s *Service) Stats(ctx context.Context, code string) (Stats, error) {
	link, err := s.store.Get(ctx, code)
	if err != nil {
		return Stats{}, translate(err)
	}
	return Stats{Link: link, Expired: link.Expired(s.now())}, nil
}

// validateTTL checks the requested lifetime against the service limits.
func (s *Service) validateTTL(ttl time.Duration) error {
	switch {
	case ttl < 0:
		return fmt.Errorf("%w: must not be negative", ErrInvalidTTL)
	case ttl > s.maxTTL:
		return fmt.Errorf("%w: must not exceed %s", ErrInvalidTTL, s.maxTTL)
	}
	return nil
}

// newCode draws a random code of the configured length. The value is reduced
// modulo 62^length; the resulting bias is negligible because the code space is
// many orders of magnitude smaller than 2^64.
func (s *Service) newCode() (string, error) {
	var buf [8]byte
	if _, err := io.ReadFull(s.random, buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint64(buf[:]) % s.codeSpace
	return base62.EncodePadded(n, s.codeLength), nil
}

// translate maps store errors to the service vocabulary.
func translate(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// validateURL checks that raw is an absolute http(s) URL and returns it
// trimmed of surrounding whitespace.
func validateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return "", fmt.Errorf("%w: must not be empty", ErrInvalidURL)
	case len(raw) > MaxURLLength:
		return "", fmt.Errorf("%w: must not exceed %d characters", ErrInvalidURL, MaxURLLength)
	}
	for _, r := range raw {
		if r <= ' ' || r == 0x7f {
			return "", fmt.Errorf("%w: must not contain spaces or control characters", ErrInvalidURL)
		}
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: malformed", ErrInvalidURL)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return "", fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	case u.Hostname() == "":
		return "", fmt.Errorf("%w: host is required", ErrInvalidURL)
	case u.User != nil:
		return "", fmt.Errorf("%w: credentials are not allowed", ErrInvalidURL)
	}
	return raw, nil
}
