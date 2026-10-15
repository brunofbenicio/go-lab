package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// testSecret is an obviously fake key used only by these tests.
var testSecret = []byte("test-only-secret-do-not-use-in-production-0123456789")

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

// clock is a manually advanced time source.
type clock struct{ t time.Time }

func (c *clock) Now() time.Time          { return c.t }
func (c *clock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newSigner(t *testing.T, c *clock, opts ...Option) *HS256 {
	t.Helper()
	h, err := NewHS256(testSecret, append([]Option{WithClock(c.Now)}, opts...)...)
	if err != nil {
		t.Fatalf("NewHS256() failed: %v", err)
	}
	return h
}

// forge builds a compact JWS from raw JSON strings, signing it with secret.
// It lets tests produce tokens the real Sign would never emit.
func forge(secret []byte, headerJSON, payloadJSON string) string {
	input := encodeSegment([]byte(headerJSON)) + "." + encodeSegment([]byte(payloadJSON))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(input))
	return input + "." + encodeSegment(mac.Sum(nil))
}

// forgeUnsigned builds a token with an empty signature, as used by the
// "none" algorithm.
func forgeUnsigned(headerJSON, payloadJSON string) string {
	return encodeSegment([]byte(headerJSON)) + "." + encodeSegment([]byte(payloadJSON)) + "."
}

const (
	hs256Header = `{"alg":"HS256","typ":"JWT"}`
	// adminPayload is a payload nobody should be able to get accepted without
	// the secret. Its exp (2100-01-01) is far in the future.
	adminPayload = `{"sub":"admin","exp":4102444800}`
)

func TestNewHS256(t *testing.T) {
	tests := []struct {
		name    string
		secret  []byte
		wantErr error
	}{
		{"exactly the minimum", make([]byte, MinSecretLength), nil},
		{"longer than the minimum", make([]byte, 64), nil},
		{"one byte short", make([]byte, MinSecretLength-1), ErrWeakSecret},
		{"empty", nil, ErrWeakSecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHS256(tt.secret)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewHS256() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewHS256CopiesTheSecret(t *testing.T) {
	secret := append([]byte(nil), testSecret...)
	h, err := NewHS256(secret, WithClock((&clock{t: t0}).Now))
	if err != nil {
		t.Fatal(err)
	}
	token, err := h.Sign(Claims{Subject: "u1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	for i := range secret {
		secret[i] = 0 // the caller wipes its copy
	}
	if _, err := h.Verify(token); err != nil {
		t.Errorf("Verify() failed after the caller modified its secret: %v", err)
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		claims Claims
	}{
		{"expiry only", Claims{ExpiresAt: t0.Add(time.Hour)}},
		{"subject and issuer", Claims{Issuer: "auth.example.com", Subject: "user-42", ExpiresAt: t0.Add(time.Hour)}},
		{"every claim", Claims{
			Issuer: "auth.example.com", Subject: "user-42", ID: "token-1",
			IssuedAt: t0, NotBefore: t0.Add(-time.Minute), ExpiresAt: t0.Add(time.Hour),
		}},
		{"non-ASCII subject", Claims{Subject: "zoë-ñ-😀", ExpiresAt: t0.Add(time.Hour)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSigner(t, &clock{t: t0})
			token, err := h.Sign(tt.claims)
			if err != nil {
				t.Fatalf("Sign() failed: %v", err)
			}
			got, err := h.Verify(token)
			if err != nil {
				t.Fatalf("Verify() failed: %v", err)
			}
			if got.Issuer != tt.claims.Issuer || got.Subject != tt.claims.Subject || got.ID != tt.claims.ID {
				t.Errorf("claims = %+v, want %+v", got, tt.claims)
			}
			for name, pair := range map[string][2]time.Time{
				"exp": {got.ExpiresAt, tt.claims.ExpiresAt},
				"nbf": {got.NotBefore, tt.claims.NotBefore},
				"iat": {got.IssuedAt, tt.claims.IssuedAt},
			} {
				if !pair[0].Equal(pair[1]) {
					t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
				}
			}
		})
	}
}

func TestSignOutputStructure(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	token, err := h.Sign(Claims{Subject: "u1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}
	if strings.ContainsAny(token, "=+/") {
		t.Errorf("token is not unpadded base64url: %q", token)
	}

	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(header) != hs256Header {
		t.Errorf("header = %s, want %s", header, hs256Header)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"sub":"u1","exp":1740834000}`; string(payload) != want {
		t.Errorf("payload = %s, want %s", payload, want)
	}

	// Recompute the signature independently of the package internals.
	mac := hmac.New(sha256.New, testSecret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); parts[2] != want {
		t.Errorf("signature = %s, want %s", parts[2], want)
	}
}

func TestSignRequiresExpiry(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	if _, err := h.Sign(Claims{Subject: "u1"}); !errors.Is(err, ErrMissingExpiry) {
		t.Errorf("Sign() error = %v, want ErrMissingExpiry", err)
	}
}

func TestSignFillsConfiguredIssuer(t *testing.T) {
	h := newSigner(t, &clock{t: t0}, WithIssuer("auth.example.com"))

	token, err := h.Sign(Claims{Subject: "u1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Issuer != "auth.example.com" {
		t.Errorf("Issuer = %q, want the configured one", got.Issuer)
	}
}

// TestVerifyRFC7515Vector checks Verify against the HS256 example of RFC 7515,
// appendix A.1, which uses a public test key.
func TestVerifyRFC7515Vector(t *testing.T) {
	const (
		key   = "AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
		token = "eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9." +
			"eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ." +
			"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	)
	secret, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		t.Fatal(err)
	}
	// The token expires at 2011-03-22T18:43:00Z.
	before := &clock{t: time.Date(2011, time.March, 22, 18, 0, 0, 0, time.UTC)}
	h, err := NewHS256(secret, WithClock(before.Now))
	if err != nil {
		t.Fatal(err)
	}

	claims, err := h.Verify(token)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}
	if claims.Issuer != "joe" || claims.ExpiresAt.Unix() != 1300819380 {
		t.Errorf("claims = %+v, want issuer joe expiring at 1300819380", claims)
	}

	before.Advance(time.Hour)
	if _, err := h.Verify(token); !errors.Is(err, ErrExpired) {
		t.Errorf("Verify() after expiry error = %v, want ErrExpired", err)
	}
}

func TestVerifyRejectsOtherAlgorithms(t *testing.T) {
	h := newSigner(t, &clock{t: t0})

	tests := []struct {
		name  string
		token string
	}{
		{"none with empty signature", forgeUnsigned(`{"alg":"none","typ":"JWT"}`, adminPayload)},
		{"none with a signature attached", forge(testSecret, `{"alg":"none"}`, adminPayload)},
		{"None", forge(testSecret, `{"alg":"None"}`, adminPayload)},
		{"NONE", forge(testSecret, `{"alg":"NONE"}`, adminPayload)},
		{"lowercase hs256", forge(testSecret, `{"alg":"hs256"}`, adminPayload)},
		{"HS384", forge(testSecret, `{"alg":"HS384"}`, adminPayload)},
		{"HS512", forge(testSecret, `{"alg":"HS512"}`, adminPayload)},
		{"RS256", forge(testSecret, `{"alg":"RS256"}`, adminPayload)},
		{"ES256", forge(testSecret, `{"alg":"ES256"}`, adminPayload)},
		{"empty algorithm", forge(testSecret, `{"alg":""}`, adminPayload)},
		{"missing algorithm", forge(testSecret, `{"typ":"JWT"}`, adminPayload)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := h.Verify(tt.token); !errors.Is(err, ErrUnsupportedAlgorithm) {
				t.Errorf("Verify() error = %v, want ErrUnsupportedAlgorithm", err)
			}
		})
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	token, err := h.Sign(Claims{Subject: "user-1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")

	otherPayload := encodeSegment([]byte(adminPayload))
	flipped := []byte(parts[2])
	if flipped[0] == 'A' {
		flipped[0] = 'B'
	} else {
		flipped[0] = 'A'
	}
	otherSecret := []byte("another-fake-secret-for-tests-only-9876543210")

	tests := []struct {
		name  string
		token string
	}{
		{"payload replaced", parts[0] + "." + otherPayload + "." + parts[2]},
		{"signature bit flipped", parts[0] + "." + parts[1] + "." + string(flipped)},
		{"signature truncated", parts[0] + "." + parts[1] + "." + parts[2][:len(parts[2])-4]},
		{"signature removed", parts[0] + "." + parts[1] + "."},
		{"signed with another secret", forge(otherSecret, hs256Header, adminPayload)},
		{"header replaced", encodeSegment([]byte(`{"alg":"HS256","typ":"JWS"}`)) + "." + parts[1] + "." + parts[2]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A truncated signature may fail to decode (malformed) or decode to
			// the wrong bytes (invalid signature); either way it is rejected.
			if _, err := h.Verify(tt.token); !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrMalformed) {
				t.Errorf("Verify() error = %v, want ErrInvalidSignature or ErrMalformed", err)
			}
		})
	}
	if _, err := h.Verify(parts[0] + "." + otherPayload + "." + parts[2]); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("tampered payload error = %v, want exactly ErrInvalidSignature", err)
	}
}

func TestVerifyRejectsMalformedTokens(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	good, err := h.Sign(Claims{Subject: "u1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, ".")

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dots", "abcdef"},
		{"one dot", "abc.def"},
		{"four segments", good + ".extra"},
		{"empty segments", ".."},
		{"header is not base64url", "!!!!." + parts[1] + "." + parts[2]},
		{"header with padding", base64.URLEncoding.EncodeToString([]byte(`{"alg":"HS256"} `)) + "." + parts[1] + "." + parts[2]},
		{"header with a line break", parts[0][:4] + "\n" + parts[0][4:] + "." + parts[1] + "." + parts[2]},
		{"header is not JSON", encodeSegment([]byte("not json")) + "." + parts[1] + "." + parts[2]},
		{"algorithm of the wrong type", forge(testSecret, `{"alg":123}`, adminPayload)},
		{"header is a JSON array", encodeSegment([]byte(`["HS256"]`)) + "." + parts[1] + "." + parts[2]},
		{"signature is not base64url", parts[0] + "." + parts[1] + ".@@@"},
		{"signature with padding", parts[0] + "." + parts[1] + "." + parts[2] + "="},
		{"payload is not JSON", forge(testSecret, hs256Header, "not json")},
		{"payload is a JSON array", forge(testSecret, hs256Header, `[1,2,3]`)},
		{"claim of the wrong type", forge(testSecret, hs256Header, `{"sub":42,"exp":1740834000}`)},
		{"exp is a string", forge(testSecret, hs256Header, `{"exp":"tomorrow"}`)},
		{"exp is negative", forge(testSecret, hs256Header, `{"exp":-1}`)},
		{"exp is absurdly large", forge(testSecret, hs256Header, `{"exp":1e300}`)},
		{"token too long", parts[0] + "." + encodeSegment([]byte(`{"sub":"`+strings.Repeat("a", MaxTokenLength)+`"}`)) + "." + parts[2]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.Verify(tt.token)
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("Verify() error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestVerifyValidatesTimes(t *testing.T) {
	exp := t0.Add(time.Hour)

	tests := []struct {
		name    string
		claims  Claims
		now     time.Time
		leeway  time.Duration
		issuer  string
		wantErr error
	}{
		{name: "valid", claims: Claims{ExpiresAt: exp}, now: t0},
		{name: "one nanosecond before expiry", claims: Claims{ExpiresAt: exp}, now: exp.Add(-time.Nanosecond)},
		{name: "exactly at expiry", claims: Claims{ExpiresAt: exp}, now: exp, wantErr: ErrExpired},
		{name: "after expiry", claims: Claims{ExpiresAt: exp}, now: exp.Add(time.Second), wantErr: ErrExpired},
		{name: "expired but within leeway", claims: Claims{ExpiresAt: exp}, now: exp.Add(30 * time.Second), leeway: time.Minute},
		{name: "expired beyond leeway", claims: Claims{ExpiresAt: exp}, now: exp.Add(time.Minute), leeway: time.Minute, wantErr: ErrExpired},
		{name: "nbf in the past", claims: Claims{ExpiresAt: exp, NotBefore: t0.Add(-time.Minute)}, now: t0},
		{name: "nbf exactly now", claims: Claims{ExpiresAt: exp, NotBefore: t0}, now: t0},
		{name: "nbf in the future", claims: Claims{ExpiresAt: exp, NotBefore: t0.Add(time.Minute)}, now: t0, wantErr: ErrNotYetValid},
		{name: "nbf in the future within leeway", claims: Claims{ExpiresAt: exp, NotBefore: t0.Add(30 * time.Second)}, now: t0, leeway: time.Minute},
		{name: "matching issuer", claims: Claims{ExpiresAt: exp, Issuer: "auth.example.com"}, now: t0, issuer: "auth.example.com"},
		{name: "different issuer", claims: Claims{ExpiresAt: exp, Issuer: "evil.example.com"}, now: t0, issuer: "auth.example.com", wantErr: ErrInvalidIssuer},
		{name: "missing issuer", claims: Claims{ExpiresAt: exp}, now: t0, issuer: "auth.example.com", wantErr: ErrInvalidIssuer},
		{name: "any issuer when none is expected", claims: Claims{ExpiresAt: exp, Issuer: "whoever"}, now: t0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &clock{t: tt.now}
			opts := []Option{WithLeeway(tt.leeway)}
			signer := newSigner(t, c)
			token, err := signer.Sign(tt.claims)
			if err != nil {
				t.Fatal(err)
			}

			verifier := newSigner(t, c, append(opts, WithIssuer(tt.issuer))...)
			_, err = verifier.Verify(token)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Verify() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyRequiresExpiry(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	token := forge(testSecret, hs256Header, `{"sub":"u1"}`)
	if _, err := h.Verify(token); !errors.Is(err, ErrMissingExpiry) {
		t.Errorf("Verify() error = %v, want ErrMissingExpiry", err)
	}
}

func TestVerifyReturnsNoClaimsOnFailure(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	token := forge(testSecret, hs256Header, `{"sub":"admin","exp":1}`)
	claims, err := h.Verify(token)
	if err == nil {
		t.Fatal("Verify() accepted an expired token")
	}
	if claims != (Claims{}) {
		t.Errorf("claims = %+v, want the zero value on failure", claims)
	}
}

func TestVerifyAcceptsDecimalNumericDates(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	token := forge(testSecret, hs256Header, `{"sub":"u1","exp":1893456000.5,"nbf":1740830399.25}`)

	claims, err := h.Verify(token)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}
	if want := time.Unix(1893456000, 500_000_000); !claims.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", claims.ExpiresAt, want)
	}
	if want := time.Unix(1740830399, 250_000_000); !claims.NotBefore.Equal(want) {
		t.Errorf("NotBefore = %v, want %v", claims.NotBefore, want)
	}
}

func TestVerifyIgnoresUnknownClaims(t *testing.T) {
	h := newSigner(t, &clock{t: t0})
	token := forge(testSecret, hs256Header, `{"sub":"u1","exp":4102444800,"role":"admin","aud":["a","b"]}`)
	claims, err := h.Verify(token)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}
	if claims.Subject != "u1" {
		t.Errorf("Subject = %q, want u1", claims.Subject)
	}
}

func TestClaimsJSON(t *testing.T) {
	t.Run("zero values are omitted", func(t *testing.T) {
		b, err := json.Marshal(Claims{Subject: "u1"})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(b), `{"sub":"u1"}`; got != want {
			t.Errorf("Marshal() = %s, want %s", got, want)
		}
	})

	t.Run("times become whole seconds", func(t *testing.T) {
		b, err := json.Marshal(Claims{ExpiresAt: time.Unix(1740834000, 999_000_000)})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(b), `{"exp":1740834000}`; got != want {
			t.Errorf("Marshal() = %s, want %s", got, want)
		}
	})

	t.Run("unmarshal resets previous contents", func(t *testing.T) {
		c := Claims{Subject: "old", ID: "old"}
		if err := json.Unmarshal([]byte(`{"iss":"new"}`), &c); err != nil {
			t.Fatal(err)
		}
		if c.Subject != "" || c.ID != "" || c.Issuer != "new" {
			t.Errorf("claims = %+v", c)
		}
	})
}
