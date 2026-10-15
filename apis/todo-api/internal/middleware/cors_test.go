package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORSSimpleRequests(t *testing.T) {
	tests := []struct {
		name            string
		opts            CORSOptions
		origin          string
		wantAllowOrigin string
		wantCredentials string
		wantVary        bool
	}{
		{name: "no origin header is untouched", opts: DefaultCORS("https://app.example.com")},
		{name: "allowed origin is echoed", opts: DefaultCORS("https://app.example.com"), origin: "https://app.example.com", wantAllowOrigin: "https://app.example.com", wantVary: true},
		{name: "other origin gets no CORS headers", opts: DefaultCORS("https://app.example.com"), origin: "https://evil.example.net", wantVary: true},
		{name: "wildcard answers star", opts: DefaultCORS("*"), origin: "https://anything.example.org", wantAllowOrigin: "*", wantVary: true},
		{
			name:            "credentials only with explicit origins",
			opts:            func() CORSOptions { o := DefaultCORS("https://app.example.com"); o.AllowCredentials = true; return o }(),
			origin:          "https://app.example.com",
			wantAllowOrigin: "https://app.example.com", wantCredentials: "true", wantVary: true,
		},
		{
			name:            "credentials never advertised with the wildcard",
			opts:            func() CORSOptions { o := DefaultCORS("*"); o.AllowCredentials = true; return o }(),
			origin:          "https://anything.example.org",
			wantAllowOrigin: "*", wantVary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/todos", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()

			CORS(tt.opts)(okHandler()).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want the handler's 200", rec.Code)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrigin {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantAllowOrigin)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != tt.wantCredentials {
				t.Errorf("Allow-Credentials = %q, want %q", got, tt.wantCredentials)
			}
			if gotVary := strings.Contains(rec.Header().Get("Vary"), "Origin"); gotVary != tt.wantVary {
				t.Errorf("Vary contains Origin = %v, want %v", gotVary, tt.wantVary)
			}
			if tt.wantAllowOrigin != "" && !strings.Contains(rec.Header().Get("Access-Control-Expose-Headers"), "X-Total-Count") {
				t.Errorf("Expose-Headers = %q, want it to include X-Total-Count", rec.Header().Get("Access-Control-Expose-Headers"))
			}
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	h := CORS(DefaultCORS("https://app.example.com"))(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/todos/1", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if called {
		t.Error("a preflight must be answered by the middleware, not forwarded")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":  "https://app.example.com",
		"Access-Control-Allow-Methods": "PATCH",
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Max-Age":       "600",
	} {
		if got := rec.Header().Get(header); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", header, got, want)
		}
	}
}

func TestCORSPreflightFromDisallowedOriginIsForwarded(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	h := CORS(DefaultCORS("https://app.example.com"))(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/todos", nil)
	req.Header.Set("Origin", "https://evil.example.net")
	req.Header.Set("Access-Control-Request-Method", "DELETE")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if !called {
		t.Error("requests from disallowed origins must fall through untouched")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Error("a disallowed origin must not learn which methods are allowed")
	}
}

func TestCORSPlainOptionsIsNotAPreflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	h := CORS(DefaultCORS("*"))(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/todos", nil)
	req.Header.Set("Origin", "https://app.example.com")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("OPTIONS without Access-Control-Request-Method is a normal request")
	}
}
