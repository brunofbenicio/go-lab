// Package middleware provides the HTTP middleware that protects routes with
// bearer tokens (RFC 6750).
//
// RequireAuth reads the Authorization header, verifies the token and stores the
// verified claims in the request context. Failures end the request with a 401
// response that carries a WWW-Authenticate challenge and the same JSON error
// envelope used by the rest of the API:
//
//	{"error": {"code": "invalid_token", "message": "token is invalid"}}
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/jwt"
)

// Verifier validates a raw token and returns its claims. *jwt.HS256 satisfies
// it.
type Verifier interface {
	Verify(token string) (jwt.Claims, error)
}

type claimsKey struct{}

// WithClaims returns a copy of ctx carrying claims. RequireAuth uses it after a
// successful verification; tests can use it to exercise protected handlers
// directly.
func WithClaims(ctx context.Context, claims jwt.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsFromContext returns the claims stored by RequireAuth. The boolean is
// false when the request did not go through the middleware.
func ClaimsFromContext(ctx context.Context) (jwt.Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(jwt.Claims)
	return claims, ok
}

// RequireAuth returns a middleware that only lets requests with a valid bearer
// token reach the wrapped handler.
func RequireAuth(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				deny(w, `Bearer realm="api"`, "missing_token", "authorization header is required")
				return
			}

			token, ok := bearerToken(header)
			if !ok {
				deny(w, `Bearer realm="api", error="invalid_request"`,
					"invalid_request", "authorization header must have the form 'Bearer <token>'")
				return
			}

			claims, err := v.Verify(token)
			switch {
			case err == nil:
				next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
			case errors.Is(err, jwt.ErrExpired):
				deny(w, `Bearer realm="api", error="invalid_token", error_description="token expired"`,
					"token_expired", "token has expired")
			default:
				// The precise reason (bad signature, wrong algorithm, ...) is
				// deliberately not revealed to the caller.
				deny(w, `Bearer realm="api", error="invalid_token", error_description="token is invalid"`,
					"invalid_token", "token is invalid")
			}
		})
	}
}

// bearerToken extracts the credentials from an "Authorization: Bearer <token>"
// header. The scheme is case-insensitive, as required by RFC 7235.
func bearerToken(header string) (string, bool) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", false
	}
	return fields[1], true
}

// deny writes a 401 response with the given challenge and JSON error.
func deny(w http.ResponseWriter, challenge, code, message string) {
	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
	w.Header().Set("WWW-Authenticate", challenge)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write(body.Bytes())
}
