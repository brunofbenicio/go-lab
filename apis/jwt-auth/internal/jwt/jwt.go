// Package jwt signs and verifies JSON Web Tokens (RFC 7519) using the HS256
// algorithm (HMAC-SHA-256) with nothing but the standard library.
//
// The verifier is deliberately strict:
//
//   - only HS256 is accepted; "none" and every other algorithm are rejected
//     before any other processing, which rules out the classic algorithm
//     confusion attacks;
//   - the signature is checked, in constant time, before the payload is parsed;
//   - tokens must carry an expiration time (exp), and exp and nbf are enforced
//     with an optional leeway for clock skew;
//   - the secret must be at least 32 bytes long (RFC 7518, section 3.2).
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	// MinSecretLength is the smallest accepted HS256 secret, in bytes. RFC 7518
	// requires a key at least as long as the hash output (256 bits).
	MinSecretLength = 32
	// MaxTokenLength bounds the size of tokens the verifier is willing to parse.
	MaxTokenLength = 8192

	algorithm = "HS256"
	// maxNumericDate is the largest accepted NumericDate: 9999-12-31T23:59:59Z.
	maxNumericDate = 253402300799
)

var (
	// ErrWeakSecret is returned by NewHS256 for secrets shorter than
	// MinSecretLength bytes.
	ErrWeakSecret = errors.New("jwt: secret is too short")
	// ErrMalformed means the token is not a well-formed compact JWS.
	ErrMalformed = errors.New("jwt: malformed token")
	// ErrUnsupportedAlgorithm means the token header names an algorithm other
	// than HS256, including "none".
	ErrUnsupportedAlgorithm = errors.New("jwt: unsupported algorithm")
	// ErrInvalidSignature means the signature does not match the token.
	ErrInvalidSignature = errors.New("jwt: invalid signature")
	// ErrMissingExpiry means the claims have no expiration time.
	ErrMissingExpiry = errors.New("jwt: missing expiration time")
	// ErrExpired means the token is at or past its expiration time.
	ErrExpired = errors.New("jwt: token expired")
	// ErrNotYetValid means the token's not-before time is still in the future.
	ErrNotYetValid = errors.New("jwt: token not valid yet")
	// ErrInvalidIssuer means the issuer claim differs from the expected one.
	ErrInvalidIssuer = errors.New("jwt: unexpected issuer")
)

// Claims are the registered claims (RFC 7519, section 4.1) supported by this
// package. A zero time or an empty string means the claim is absent.
type Claims struct {
	// Issuer (iss) identifies who issued the token.
	Issuer string
	// Subject (sub) identifies who the token is about.
	Subject string
	// ExpiresAt (exp) is when the token stops being valid. It is required.
	ExpiresAt time.Time
	// NotBefore (nbf) is when the token starts being valid.
	NotBefore time.Time
	// IssuedAt (iat) is when the token was issued.
	IssuedAt time.Time
	// ID (jti) uniquely identifies the token.
	ID string
}

// wireClaims is the JSON representation of Claims.
type wireClaims struct {
	Issuer    string       `json:"iss,omitempty"`
	Subject   string       `json:"sub,omitempty"`
	ExpiresAt *numericDate `json:"exp,omitempty"`
	NotBefore *numericDate `json:"nbf,omitempty"`
	IssuedAt  *numericDate `json:"iat,omitempty"`
	ID        string       `json:"jti,omitempty"`
}

// numericDate is a JSON NumericDate: seconds since the Unix epoch. It is
// written as an integer and read as an integer or a decimal number.
type numericDate struct{ time.Time }

func toDate(t time.Time) *numericDate {
	if t.IsZero() {
		return nil
	}
	return &numericDate{t}
}

func fromDate(d *numericDate) time.Time {
	if d == nil {
		return time.Time{}
	}
	return d.Time
}

func (d numericDate) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "%d", d.Unix()), nil
}

func (d *numericDate) UnmarshalJSON(b []byte) error {
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("NumericDate must be a number: %w", err)
	}
	if math.IsNaN(f) || f < 0 || f > maxNumericDate {
		return errors.New("NumericDate out of range")
	}
	sec, frac := math.Modf(f)
	d.Time = time.Unix(int64(sec), int64(frac*1e9)).UTC()
	return nil
}

// MarshalJSON encodes the claims with the registered claim names. Zero values
// are omitted and times are written as whole seconds since the Unix epoch.
func (c Claims) MarshalJSON() ([]byte, error) {
	return json.Marshal(wireClaims{
		Issuer:    c.Issuer,
		Subject:   c.Subject,
		ExpiresAt: toDate(c.ExpiresAt),
		NotBefore: toDate(c.NotBefore),
		IssuedAt:  toDate(c.IssuedAt),
		ID:        c.ID,
	})
}

// UnmarshalJSON decodes claims written by MarshalJSON or by any JWT issuer.
// Unknown claims are ignored.
func (c *Claims) UnmarshalJSON(b []byte) error {
	var w wireClaims
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*c = Claims{
		Issuer:    w.Issuer,
		Subject:   w.Subject,
		ExpiresAt: fromDate(w.ExpiresAt),
		NotBefore: fromDate(w.NotBefore),
		IssuedAt:  fromDate(w.IssuedAt),
		ID:        w.ID,
	}
	return nil
}

// HS256 signs and verifies tokens with a shared secret. It is safe for
// concurrent use.
type HS256 struct {
	secret []byte
	now    func() time.Time
	leeway time.Duration
	issuer string
}

// Option customizes an HS256.
type Option func(*HS256)

// WithClock replaces the time source used to validate exp and nbf. A nil
// function is ignored.
func WithClock(now func() time.Time) Option {
	return func(h *HS256) {
		if now != nil {
			h.now = now
		}
	}
}

// WithLeeway tolerates up to d of clock skew when checking exp and nbf.
// Negative values are treated as zero.
func WithLeeway(d time.Duration) Option {
	return func(h *HS256) { h.leeway = max(d, 0) }
}

// WithIssuer sets the expected issuer. Verify then rejects tokens issued by
// anyone else, and Sign fills the issuer claim when it is empty.
func WithIssuer(issuer string) Option {
	return func(h *HS256) { h.issuer = issuer }
}

// NewHS256 returns an HS256 using secret. The secret is copied. It returns
// ErrWeakSecret if the secret is shorter than MinSecretLength bytes.
func NewHS256(secret []byte, opts ...Option) (*HS256, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("%w: need at least %d bytes, got %d", ErrWeakSecret, MinSecretLength, len(secret))
	}
	h := &HS256{
		secret: append([]byte(nil), secret...),
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

// headerSegment is the base64url encoding of the only header this package
// produces: {"alg":"HS256","typ":"JWT"}.
var headerSegment = encodeSegment([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Sign returns the compact serialization of a token carrying claims. It
// returns ErrMissingExpiry if claims.ExpiresAt is zero: tokens that never
// expire are refused on purpose.
func (h *HS256) Sign(claims Claims) (string, error) {
	if claims.ExpiresAt.IsZero() {
		return "", ErrMissingExpiry
	}
	if claims.Issuer == "" {
		claims.Issuer = h.issuer
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: encode claims: %w", err)
	}
	signingInput := headerSegment + "." + encodeSegment(payload)
	return signingInput + "." + encodeSegment(h.sum(signingInput)), nil
}

// Verify checks the token and returns its claims.
//
// The checks run in this order: structure (ErrMalformed), algorithm
// (ErrUnsupportedAlgorithm), signature (ErrInvalidSignature), payload
// (ErrMalformed), then the claims: expiration (ErrMissingExpiry, ErrExpired),
// not-before (ErrNotYetValid) and issuer (ErrInvalidIssuer, only when an
// issuer was configured). Claims are never returned for an invalid token.
func (h *HS256) Verify(token string) (Claims, error) {
	if len(token) > MaxTokenLength {
		return Claims{}, fmt.Errorf("%w: token is too long", ErrMalformed)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: expected 3 segments, got %d", ErrMalformed, len(parts))
	}

	rawHeader, err := decodeSegment(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: header: %v", ErrMalformed, err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		return Claims{}, fmt.Errorf("%w: header is not valid JSON", ErrMalformed)
	}
	// Exact, case-sensitive comparison: "none", "None", "hs256" and every other
	// algorithm are refused here, before the signature is even looked at.
	if header.Alg != algorithm {
		return Claims{}, fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, header.Alg)
	}

	signature, err := decodeSegment(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: signature: %v", ErrMalformed, err)
	}
	if !hmac.Equal(signature, h.sum(parts[0]+"."+parts[1])) {
		return Claims{}, ErrInvalidSignature
	}

	rawPayload, err := decodeSegment(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: payload: %v", ErrMalformed, err)
	}
	var claims Claims
	if err := json.Unmarshal(rawPayload, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: payload: %v", ErrMalformed, err)
	}

	if err := h.validate(claims); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

// validate enforces exp, nbf and the issuer.
func (h *HS256) validate(c Claims) error {
	now := h.now()
	switch {
	case c.ExpiresAt.IsZero():
		return ErrMissingExpiry
	case !now.Before(c.ExpiresAt.Add(h.leeway)):
		return ErrExpired
	case !c.NotBefore.IsZero() && now.Add(h.leeway).Before(c.NotBefore):
		return ErrNotYetValid
	case h.issuer != "" && c.Issuer != h.issuer:
		return ErrInvalidIssuer
	}
	return nil
}

// sum returns the HMAC-SHA-256 of the signing input.
func (h *HS256) sum(signingInput string) []byte {
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}

func encodeSegment(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeSegment decodes an unpadded base64url segment. It is stricter than the
// standard decoder: line breaks, padding and non-canonical trailing bits are
// all rejected.
func decodeSegment(s string) ([]byte, error) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, errors.New("line breaks are not allowed")
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, errors.New("not valid unpadded base64url")
	}
	return b, nil
}
