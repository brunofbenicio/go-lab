package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireAPIKey protects every request that can change state (anything but
// GET, HEAD and OPTIONS). The key is read from the X-API-Key header or from an
// "Authorization: Bearer <key>" header.
//
// With no keys configured the middleware lets everything through, which keeps
// local development friction-free; production deployments should always set one.
func RequireAPIKey(keys ...string) Middleware {
	digests := make([][sha256.Size]byte, 0, len(keys))
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			digests = append(digests, sha256.Sum256([]byte(k)))
		}
	}

	return func(next http.Handler) http.Handler {
		if len(digests) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) || validKey(digests, presentedKey(r)) {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("WWW-Authenticate", `Bearer realm="todo-api"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"a valid API key is required for this operation"}}` + "\n"))
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// presentedKey extracts the credential sent by the client, if any.
func presentedKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	if auth := r.Header.Get("Authorization"); len(auth) > len("Bearer ") && strings.EqualFold(auth[:len("Bearer ")], "Bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

// validKey compares in constant time. Hashing both sides first hides the key
// length, and the loop never exits early so timing does not reveal which
// configured key matched.
func validKey(digests [][sha256.Size]byte, presented string) bool {
	if presented == "" {
		return false
	}
	got := sha256.Sum256([]byte(presented))
	match := 0
	for _, d := range digests {
		match |= subtle.ConstantTimeCompare(got[:], d[:])
	}
	return match == 1
}
