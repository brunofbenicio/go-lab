// Package uuid creates and parses version 4 (random) UUIDs as defined by
// RFC 4122, using crypto/rand. It has no dependencies beyond the standard
// library.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// UUID is a 128-bit universally unique identifier.
type UUID [16]byte

// Nil is the all-zero UUID.
var Nil UUID

// ErrInvalid is returned by Parse for malformed input.
var ErrInvalid = errors.New("uuid: invalid format")

// New returns a random (version 4) UUID. It panics only if the operating
// system's random source fails, which is not recoverable.
func New() UUID {
	u, err := NewFromReader(rand.Reader)
	if err != nil {
		panic(err)
	}
	return u
}

// NewFromReader builds a version 4 UUID from 16 bytes read from r, setting the
// version and variant bits. A deterministic reader makes it testable.
func NewFromReader(r io.Reader) (UUID, error) {
	var u UUID
	if _, err := io.ReadFull(r, u[:]); err != nil {
		return Nil, err
	}
	u[6] = u[6]&0x0f | 0x40 // version 4
	u[8] = u[8]&0x3f | 0x80 // variant RFC 4122
	return u, nil
}

// Parse decodes the canonical 36-character form
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx, in either letter case.
func Parse(s string) (UUID, error) {
	var u UUID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return Nil, ErrInvalid
	}
	hexOnly := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:]
	if _, err := hex.Decode(u[:], []byte(hexOnly)); err != nil {
		return Nil, ErrInvalid
	}
	return u, nil
}

// String returns the canonical lowercase form.
func (u UUID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])
	return string(buf[:])
}

// IsNil reports whether u is the all-zero UUID.
func (u UUID) IsNil() bool { return u == Nil }

// Version returns the version number stored in the UUID (4 for New).
func (u UUID) Version() int { return int(u[6] >> 4) }

// MarshalText implements encoding.TextMarshaler, so UUIDs serialize as
// strings in JSON.
func (u UUID) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (u *UUID) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}
