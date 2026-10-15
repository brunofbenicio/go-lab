package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSOptions configures the CORS middleware.
type CORSOptions struct {
	// AllowedOrigins lists the origins allowed to call the API. "*" allows any
	// origin (and then credentials are never advertised).
	AllowedOrigins []string
	// AllowedMethods is sent in answers to preflight requests.
	AllowedMethods []string
	// AllowedHeaders is sent in answers to preflight requests.
	AllowedHeaders []string
	// ExposedHeaders lists response headers browsers may read from JavaScript.
	ExposedHeaders []string
	// AllowCredentials adds Access-Control-Allow-Credentials. It is ignored
	// for the "*" wildcard, which browsers refuse to combine with credentials.
	AllowCredentials bool
	// MaxAgeSeconds is how long browsers may cache a preflight answer.
	MaxAgeSeconds int
}

// DefaultCORS returns options suited to a public JSON API.
func DefaultCORS(allowedOrigins ...string) CORSOptions {
	return CORSOptions{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-API-Key", RequestIDHeader},
		ExposedHeaders: []string{"X-Total-Count", RequestIDHeader},
		MaxAgeSeconds:  600,
	}
}

// CORS implements Cross-Origin Resource Sharing. Requests from origins that
// are not allowed are not rejected, they simply get no CORS headers, so the
// browser blocks the response while curl and servers keep working.
func CORS(opts CORSOptions) Middleware {
	allowAny := false
	allowed := make(map[string]bool, len(opts.AllowedOrigins))
	for _, o := range opts.AllowedOrigins {
		if o == "*" {
			allowAny = true
		}
		allowed[o] = true
	}
	methods := strings.Join(opts.AllowedMethods, ", ")
	headers := strings.Join(opts.AllowedHeaders, ", ")
	exposed := strings.Join(opts.ExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r) // not a cross-origin request
				return
			}

			// The answer depends on the Origin header, so caches must key on it.
			w.Header().Add("Vary", "Origin")

			if !allowAny && !allowed[origin] {
				next.ServeHTTP(w, r)
				return
			}

			if allowAny {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if opts.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
			if exposed != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposed)
			}

			isPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
			if !isPreflight {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			if methods != "" {
				w.Header().Set("Access-Control-Allow-Methods", methods)
			}
			if headers != "" {
				w.Header().Set("Access-Control-Allow-Headers", headers)
			}
			if opts.MaxAgeSeconds > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(opts.MaxAgeSeconds))
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}
