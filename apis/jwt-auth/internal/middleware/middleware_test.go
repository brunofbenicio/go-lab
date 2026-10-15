package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/jwt"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

// fakeVerifier records the token it receives and returns a canned result.
type fakeVerifier struct {
	claims jwt.Claims
	err    error

	calls int
	token string
}

func (f *fakeVerifier) Verify(token string) (jwt.Claims, error) {
	f.calls++
	f.token = token
	return f.claims, f.err
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestRequireAuthRejectsRequests(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		verifyErr     error
		wantCode      string
		wantChallenge string
		wantVerifies  bool
	}{
		{
			name:          "no header",
			wantCode:      "missing_token",
			wantChallenge: `Bearer realm="api"`,
		},
		{
			name:          "basic scheme",
			header:        "Basic dXNlcjpwYXNz",
			wantCode:      "invalid_request",
			wantChallenge: `Bearer realm="api", error="invalid_request"`,
		},
		{
			name:          "token without a scheme",
			header:        "abc.def.ghi",
			wantCode:      "invalid_request",
			wantChallenge: `Bearer realm="api", error="invalid_request"`,
		},
		{
			name:          "scheme without a token",
			header:        "Bearer",
			wantCode:      "invalid_request",
			wantChallenge: `Bearer realm="api", error="invalid_request"`,
		},
		{
			name:          "scheme with a trailing space only",
			header:        "Bearer ",
			wantCode:      "invalid_request",
			wantChallenge: `Bearer realm="api", error="invalid_request"`,
		},
		{
			name:          "two tokens",
			header:        "Bearer abc def",
			wantCode:      "invalid_request",
			wantChallenge: `Bearer realm="api", error="invalid_request"`,
		},
		{
			name:          "bearer-like prefix",
			header:        "Bearerabc",
			wantCode:      "invalid_request",
			wantChallenge: `Bearer realm="api", error="invalid_request"`,
		},
		{
			name:          "expired token",
			header:        "Bearer some.token.value",
			verifyErr:     jwt.ErrExpired,
			wantCode:      "token_expired",
			wantChallenge: `Bearer realm="api", error="invalid_token", error_description="token expired"`,
			wantVerifies:  true,
		},
		{
			name:          "wrapped expiry is still an expiry",
			header:        "Bearer some.token.value",
			verifyErr:     errors.Join(errors.New("context"), jwt.ErrExpired),
			wantCode:      "token_expired",
			wantChallenge: `Bearer realm="api", error="invalid_token", error_description="token expired"`,
			wantVerifies:  true,
		},
		{
			name:          "bad signature",
			header:        "Bearer some.token.value",
			verifyErr:     jwt.ErrInvalidSignature,
			wantCode:      "invalid_token",
			wantChallenge: `Bearer realm="api", error="invalid_token", error_description="token is invalid"`,
			wantVerifies:  true,
		},
		{
			name:          "unsupported algorithm",
			header:        "Bearer some.token.value",
			verifyErr:     jwt.ErrUnsupportedAlgorithm,
			wantCode:      "invalid_token",
			wantChallenge: `Bearer realm="api", error="invalid_token", error_description="token is invalid"`,
			wantVerifies:  true,
		},
		{
			name:          "not yet valid",
			header:        "Bearer some.token.value",
			verifyErr:     jwt.ErrNotYetValid,
			wantCode:      "invalid_token",
			wantChallenge: `Bearer realm="api", error="invalid_token", error_description="token is invalid"`,
			wantVerifies:  true,
		},
		{
			name:          "unexpected verifier failure",
			header:        "Bearer some.token.value",
			verifyErr:     errors.New("boom"),
			wantCode:      "invalid_token",
			wantChallenge: `Bearer realm="api", error="invalid_token", error_description="token is invalid"`,
			wantVerifies:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &fakeVerifier{err: tt.verifyErr}
			nextCalled := false
			handler := RequireAuth(v)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				nextCalled = true
			}))

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if nextCalled {
				t.Error("the protected handler ran for a rejected request")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != tt.wantChallenge {
				t.Errorf("WWW-Authenticate = %q, want %q", got, tt.wantChallenge)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not valid JSON: %v\n%s", err, rec.Body.String())
			}
			if body.Error.Code != tt.wantCode || body.Error.Message == "" {
				t.Errorf("error = %+v, want code %q with a message", body.Error, tt.wantCode)
			}
			if (v.calls == 1) != tt.wantVerifies || v.calls > 1 {
				t.Errorf("verifier called %d times, want verified=%v", v.calls, tt.wantVerifies)
			}
		})
	}
}

func TestRequireAuthAcceptsValidTokens(t *testing.T) {
	tests := []struct {
		name   string
		header string
		token  string
	}{
		{"canonical", "Bearer abc.def.ghi", "abc.def.ghi"},
		{"lowercase scheme", "bearer abc.def.ghi", "abc.def.ghi"},
		{"uppercase scheme", "BEARER abc.def.ghi", "abc.def.ghi"},
		{"extra spaces", "Bearer    abc.def.ghi  ", "abc.def.ghi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := jwt.Claims{Subject: "user-1", ExpiresAt: t0.Add(time.Hour)}
			v := &fakeVerifier{claims: want}

			var got jwt.Claims
			var found bool
			handler := RequireAuth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, found = ClaimsFromContext(r.Context())
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", rec.Code)
			}
			if v.token != tt.token {
				t.Errorf("verifier received %q, want %q", v.token, tt.token)
			}
			if !found || got.Subject != want.Subject || !got.ExpiresAt.Equal(want.ExpiresAt) {
				t.Errorf("claims in context = %+v (found %v), want %+v", got, found, want)
			}
		})
	}
}

func TestRequireAuthWithRealVerifier(t *testing.T) {
	now := t0
	secret := []byte("test-only-secret-do-not-use-in-production-0123456789")
	signer, err := jwt.NewHS256(secret, jwt.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.Sign(jwt.Claims{Subject: "user-1", ExpiresAt: t0.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	handler := RequireAuth(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFromContext(r.Context())
		_, _ = w.Write([]byte(claims.Subject))
	}))

	call := func(authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", authorization)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := call("Bearer " + token); rec.Code != http.StatusOK || rec.Body.String() != "user-1" {
		t.Errorf("valid token: status %d, body %q", rec.Code, rec.Body.String())
	}
	if rec := call("Bearer " + token + "x"); rec.Code != http.StatusUnauthorized {
		t.Errorf("tampered token: status %d, want 401", rec.Code)
	}

	now = now.Add(time.Hour)
	rec := call("Bearer " + token)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "token_expired") {
		t.Errorf("expired token: status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestClaimsFromContext(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		if _, ok := ClaimsFromContext(context.Background()); ok {
			t.Error("ClaimsFromContext() found claims in an empty context")
		}
	})
	t.Run("present", func(t *testing.T) {
		want := jwt.Claims{Subject: "u1"}
		got, ok := ClaimsFromContext(WithClaims(context.Background(), want))
		if !ok || got.Subject != "u1" {
			t.Errorf("ClaimsFromContext() = %+v, %v", got, ok)
		}
	})
	t.Run("a foreign value under another key is ignored", func(t *testing.T) {
		type otherKey struct{}
		ctx := context.WithValue(context.Background(), otherKey{}, jwt.Claims{Subject: "u1"})
		if _, ok := ClaimsFromContext(ctx); ok {
			t.Error("ClaimsFromContext() accepted a value stored under a different key")
		}
	})
}
