// Package signature signs and verifies webhook deliveries with HMAC-SHA-256.
//
// A signature is transported as the string "sha256=<hex>", the same shape used
// by many webhook providers. What gets signed is not just the body but the
// delivery ID, the timestamp and the body together:
//
//	<id>.<timestamp>.<body>
//
// Covering the ID and the timestamp matters: if only the body were signed, a
// captured request could be replayed with a fresh timestamp, or with a new
// delivery ID to slip past the duplicate detection.
package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// Prefix introduces the hex-encoded digest in a signature header.
const Prefix = "sha256="

var (
	// ErrEmptySecret is returned by Verify when the secret is empty: an empty
	// key would let anyone forge signatures.
	ErrEmptySecret = errors.New("signature: secret must not be empty")
	// ErrMissing is returned when the signature header is empty.
	ErrMissing = errors.New("signature: signature is missing")
	// ErrMalformed is returned when the header is not "sha256=" followed by 64
	// hexadecimal digits.
	ErrMalformed = errors.New("signature: malformed signature")
	// ErrMismatch is returned when the signature is well formed but does not
	// match the message.
	ErrMismatch = errors.New("signature: signature does not match")
)

// digestHexLength is the length of a hex-encoded SHA-256 digest.
const digestHexLength = 2 * sha256.Size

// Message returns the bytes that are signed for a delivery:
// "<id>.<timestamp>.<body>".
func Message(id, timestamp string, body []byte) []byte {
	msg := make([]byte, 0, len(id)+len(timestamp)+len(body)+2)
	msg = append(msg, id...)
	msg = append(msg, '.')
	msg = append(msg, timestamp...)
	msg = append(msg, '.')
	return append(msg, body...)
}

// Sign returns "sha256=<hex>", where the digest is the HMAC-SHA-256 of message
// under secret. Senders (and tests) use it to produce the signature header.
func Sign(secret, message []byte) string {
	return Prefix + hex.EncodeToString(digest(secret, message))
}

// Verify checks a signature header against message. The digests are compared in
// constant time, so the check does not leak how many leading bytes matched.
//
// It returns ErrEmptySecret, ErrMissing, ErrMalformed or ErrMismatch. Callers
// should treat all of them as "not authenticated" and avoid telling the sender
// which one it was.
func Verify(secret, message []byte, header string) error {
	if len(secret) == 0 {
		return ErrEmptySecret
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return ErrMissing
	}

	encoded, ok := strings.CutPrefix(header, Prefix)
	if !ok || len(encoded) != digestHexLength {
		return ErrMalformed
	}
	got, err := hex.DecodeString(encoded)
	if err != nil {
		return ErrMalformed
	}

	if !hmac.Equal(got, digest(secret, message)) {
		return ErrMismatch
	}
	return nil
}

func digest(secret, message []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	return mac.Sum(nil)
}
