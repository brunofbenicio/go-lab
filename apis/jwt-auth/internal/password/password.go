// Package password hashes and verifies passwords with PBKDF2-HMAC-SHA-256,
// using only the standard library (crypto/pbkdf2, available since Go 1.24).
//
// Hashes are self-describing strings in the PHC style:
//
//	$pbkdf2-sha256$i=600000$<salt>$<hash>
//
// where salt and hash are unpadded standard base64. Because the iteration count
// and the salt travel with the hash, the cost can be raised in the future
// without invalidating existing hashes.
package password

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	// DefaultIterations follows the OWASP recommendation for PBKDF2-HMAC-SHA256
	// (600,000 iterations).
	DefaultIterations = 600_000
	// DefaultSaltLength is the size, in bytes, of newly drawn salts.
	DefaultSaltLength = 16
	// MinSaltLength is the smallest salt a Hasher may be configured with.
	MinSaltLength = 8
	// MaxPasswordLength is the longest password, in bytes, that is hashed.
	MaxPasswordLength = 1024
	// MaxIterations is the highest iteration count Verify accepts from a stored
	// hash. It keeps a corrupted or hostile hash from stalling the server.
	MaxIterations = 10_000_000

	scheme = "pbkdf2-sha256"
	// keyLength is the size of the derived key: one SHA-256 block.
	keyLength = sha256.Size
)

var (
	// ErrEmptyPassword is returned by Hash for an empty password.
	ErrEmptyPassword = errors.New("password: password must not be empty")
	// ErrPasswordTooLong is returned by Hash for passwords above
	// MaxPasswordLength bytes.
	ErrPasswordTooLong = errors.New("password: password is too long")
	// ErrInvalidHash is returned by Verify when the stored hash cannot be parsed.
	ErrInvalidHash = errors.New("password: invalid hash")
)

// Hasher creates and checks password hashes. It is safe for concurrent use.
type Hasher struct {
	iterations int
	saltLength int
	random     io.Reader
}

// Option customizes a Hasher.
type Option func(*Hasher)

// WithIterations sets the PBKDF2 iteration count for new hashes. Values below
// DefaultIterations weaken the hashes; lower them only in tests.
func WithIterations(n int) Option {
	return func(h *Hasher) { h.iterations = n }
}

// WithSaltLength sets the salt size, in bytes, for new hashes.
func WithSaltLength(n int) Option {
	return func(h *Hasher) { h.saltLength = n }
}

// WithRandom replaces the source of salts. By default crypto/rand is used. A
// nil reader is ignored.
func WithRandom(r io.Reader) Option {
	return func(h *Hasher) {
		if r != nil {
			h.random = r
		}
	}
}

// NewHasher returns a Hasher with the given options applied over the defaults.
// It fails when an option is out of range.
func NewHasher(opts ...Option) (*Hasher, error) {
	h := &Hasher{
		iterations: DefaultIterations,
		saltLength: DefaultSaltLength,
		random:     rand.Reader,
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.iterations < 1 || h.iterations > MaxIterations {
		return nil, fmt.Errorf("password: iterations must be between 1 and %d, got %d", MaxIterations, h.iterations)
	}
	if h.saltLength < MinSaltLength {
		return nil, fmt.Errorf("password: salt must have at least %d bytes, got %d", MinSaltLength, h.saltLength)
	}
	return h, nil
}

// Hash returns the encoded hash of password under a fresh random salt.
func (h *Hasher) Hash(password string) (string, error) {
	switch {
	case password == "":
		return "", ErrEmptyPassword
	case len(password) > MaxPasswordLength:
		return "", ErrPasswordTooLong
	}

	salt := make([]byte, h.saltLength)
	if _, err := io.ReadFull(h.random, salt); err != nil {
		return "", fmt.Errorf("password: generate salt: %w", err)
	}
	key, err := derive(password, salt, h.iterations)
	if err != nil {
		return "", err
	}
	return encode(h.iterations, salt, key), nil
}

// Verify reports whether password matches the encoded hash. The comparison
// takes constant time. The iteration count and salt come from the hash itself,
// not from the Hasher's settings.
//
// A wrong password is not an error: Verify returns (false, nil). A hash that
// cannot be parsed yields ErrInvalidHash.
func (h *Hasher) Verify(password, encoded string) (bool, error) {
	iterations, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}
	if password == "" || len(password) > MaxPasswordLength {
		return false, nil
	}
	got, err := derive(password, salt, iterations)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// derive runs PBKDF2-HMAC-SHA-256.
func derive(password string, salt []byte, iterations int) ([]byte, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
	if err != nil {
		return nil, fmt.Errorf("password: derive key: %w", err)
	}
	return key, nil
}

func encode(iterations int, salt, key []byte) string {
	b64 := base64.RawStdEncoding
	return "$" + scheme + "$i=" + strconv.Itoa(iterations) + "$" + b64.EncodeToString(salt) + "$" + b64.EncodeToString(key)
}

// decode parses an encoded hash into its parts.
func decode(encoded string) (iterations int, salt, key []byte, err error) {
	// "$pbkdf2-sha256$i=N$salt$key" splits into ["", scheme, "i=N", salt, key].
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "" || parts[1] != scheme {
		return 0, nil, nil, fmt.Errorf("%w: unrecognized format", ErrInvalidHash)
	}

	n, ok := strings.CutPrefix(parts[2], "i=")
	if !ok {
		return 0, nil, nil, fmt.Errorf("%w: missing iteration count", ErrInvalidHash)
	}
	iterations, convErr := strconv.Atoi(n)
	if convErr != nil || iterations < 1 || iterations > MaxIterations || n != strconv.Itoa(iterations) {
		return 0, nil, nil, fmt.Errorf("%w: bad iteration count", ErrInvalidHash)
	}

	b64 := base64.RawStdEncoding.Strict()
	if salt, err = b64.DecodeString(parts[3]); err != nil || len(salt) == 0 {
		return 0, nil, nil, fmt.Errorf("%w: bad salt", ErrInvalidHash)
	}
	if key, err = b64.DecodeString(parts[4]); err != nil || len(key) != keyLength {
		return 0, nil, nil, fmt.Errorf("%w: bad key", ErrInvalidHash)
	}
	return iterations, salt, key, nil
}
