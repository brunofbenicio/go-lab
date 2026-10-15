package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/shortener"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/store"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

// fakeService is a Service whose behavior is set per test.
type fakeService struct {
	shorten func(shortener.ShortenInput) (store.Link, error)
	resolve func(code string) (store.Link, error)
	stats   func(code string) (shortener.Stats, error)

	calls int
}

func (f *fakeService) Shorten(_ context.Context, in shortener.ShortenInput) (store.Link, error) {
	f.calls++
	return f.shorten(in)
}

func (f *fakeService) Resolve(_ context.Context, code string) (store.Link, error) {
	f.calls++
	return f.resolve(code)
}

func (f *fakeService) Stats(_ context.Context, code string) (shortener.Stats, error) {
	f.calls++
	return f.stats(code)
}

func newMux(svc Service, opts ...Option) *http.ServeMux {
	mux := http.NewServeMux()
	New(svc, opts...).Register(mux)
	return mux
}

// do sends a request through handler and returns the recorded response.
func do(t *testing.T, handler http.Handler, method, target, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
	return v
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := decode[errorBody](t, rec)
	if body.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q (message %q)", body.Error.Code, wantCode, body.Error.Message)
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	mux := newMux(&fakeService{})
	for _, pattern := range Routes() {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("malformed pattern %q", pattern)
		}
		path = strings.ReplaceAll(path, "{code}", "abc1234")

		req := httptest.NewRequest(method, path, nil)
		if _, got := mux.Handler(req); got != pattern {
			t.Errorf("%s %s matched pattern %q, want %q", method, path, got, pattern)
		}
	}
}

func TestCreateLink(t *testing.T) {
	created := store.Link{Code: "aB3dE9x", URL: "https://example.com/some/page", CreatedAt: t0}
	withExpiry := created
	withExpiry.ExpiresAt = t0.Add(time.Hour)

	tests := []struct {
		name          string
		body          string
		result        store.Link
		wantInput     shortener.ShortenInput
		wantExpiresAt bool
	}{
		{
			name:      "url only",
			body:      `{"url":"https://example.com/some/page"}`,
			result:    created,
			wantInput: shortener.ShortenInput{URL: "https://example.com/some/page"},
		},
		{
			name:          "url with ttl",
			body:          `{"url":"https://example.com/some/page","ttl_seconds":3600}`,
			result:        withExpiry,
			wantInput:     shortener.ShortenInput{URL: "https://example.com/some/page", TTL: time.Hour},
			wantExpiresAt: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got shortener.ShortenInput
			svc := &fakeService{shorten: func(in shortener.ShortenInput) (store.Link, error) {
				got = in
				return tt.result, nil
			}}
			rec := do(t, newMux(svc, WithBaseURL("https://sho.example.com/")), http.MethodPost,
				"/api/links", "application/json", tt.body)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
			}
			if got != tt.wantInput {
				t.Errorf("service input = %+v, want %+v", got, tt.wantInput)
			}
			if loc, want := rec.Header().Get("Location"), "https://sho.example.com/api/links/aB3dE9x/stats"; loc != want {
				t.Errorf("Location = %q, want %q", loc, want)
			}

			body := decode[linkResponse](t, rec)
			if body.Code != "aB3dE9x" || body.ShortURL != "https://sho.example.com/aB3dE9x" || body.URL != created.URL {
				t.Errorf("unexpected body: %+v", body)
			}
			if !body.CreatedAt.Equal(t0) {
				t.Errorf("created_at = %v, want %v", body.CreatedAt, t0)
			}
			if (body.ExpiresAt != nil) != tt.wantExpiresAt {
				t.Errorf("expires_at present = %v, want %v", body.ExpiresAt != nil, tt.wantExpiresAt)
			}
			if tt.wantExpiresAt && !strings.Contains(rec.Body.String(), `"expires_at":"2025-03-01T13:00:00Z"`) {
				t.Errorf("expires_at not serialized as RFC 3339: %s", rec.Body.String())
			}
			if !tt.wantExpiresAt && strings.Contains(rec.Body.String(), "expires_at") {
				t.Errorf("expires_at should be omitted: %s", rec.Body.String())
			}
		})
	}
}

func TestCreateLinkRejectsBadRequests(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{"missing content type", "", `{"url":"https://example.com"}`, 415, "unsupported_media_type"},
		{"form content type", "application/x-www-form-urlencoded", `url=https://example.com`, 415, "unsupported_media_type"},
		{"charset parameter is accepted and the body is decoded", "application/json; charset=utf-8", `{`, 400, "invalid_json"},
		{"empty body", "application/json", ``, 400, "invalid_json"},
		{"truncated JSON", "application/json", `{"url":"https://exa`, 400, "invalid_json"},
		{"not JSON", "application/json", `hello`, 400, "invalid_json"},
		{"array instead of object", "application/json", `["https://example.com"]`, 400, "invalid_json"},
		{"unknown field", "application/json", `{"url":"https://example.com","color":"red"}`, 400, "invalid_json"},
		{"wrong field type", "application/json", `{"url":42}`, 400, "invalid_json"},
		{"ttl as string", "application/json", `{"url":"https://example.com","ttl_seconds":"60"}`, 400, "invalid_json"},
		{"ttl as fraction", "application/json", `{"url":"https://example.com","ttl_seconds":1.5}`, 400, "invalid_json"},
		{"two JSON values", "application/json", `{"url":"https://example.com"}{"url":"https://example.org"}`, 400, "invalid_json"},
		{"trailing garbage", "application/json", `{"url":"https://example.com"} x`, 400, "invalid_json"},
		{"zero ttl", "application/json", `{"url":"https://example.com","ttl_seconds":0}`, 422, "invalid_ttl"},
		{"negative ttl", "application/json", `{"url":"https://example.com","ttl_seconds":-5}`, 422, "invalid_ttl"},
		{"body too large", "application/json", `{"url":"https://example.com/` + strings.Repeat("a", MaxBodyBytes) + `"}`, 413, "payload_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{}
			rec := do(t, newMux(svc), http.MethodPost, "/api/links", tt.contentType, tt.body)
			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if svc.calls != 0 {
				t.Errorf("service was called %d times for a rejected request", svc.calls)
			}
		})
	}
}

func TestCreateLinkSaturatesHugeTTL(t *testing.T) {
	var got shortener.ShortenInput
	svc := &fakeService{shorten: func(in shortener.ShortenInput) (store.Link, error) {
		got = in
		return store.Link{}, shortener.ErrInvalidTTL
	}}
	rec := do(t, newMux(svc), http.MethodPost, "/api/links", "application/json",
		`{"url":"https://example.com","ttl_seconds":9223372036854775807}`)

	assertError(t, rec, http.StatusUnprocessableEntity, "invalid_ttl")
	if got.TTL <= 0 {
		t.Errorf("TTL = %v; a huge value must saturate, not overflow into a negative or zero TTL", got.TTL)
	}
}

func TestCreateLinkMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"invalid URL", fmt.Errorf("%w: must not be empty", shortener.ErrInvalidURL), 422, "invalid_url", "invalid URL"},
		{"invalid TTL", shortener.ErrInvalidTTL, 422, "invalid_ttl", "invalid TTL"},
		{"code generation exhausted", shortener.ErrCodeGeneration, 503, "code_generation_failed", ""},
		{"unexpected failure", errors.New("connection to db-secret-host refused"), 500, "internal_error", "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{shorten: func(shortener.ShortenInput) (store.Link, error) {
				return store.Link{}, tt.err
			}}
			rec := do(t, newMux(svc), http.MethodPost, "/api/links", "application/json", `{"url":"https://example.com"}`)
			assertError(t, rec, tt.wantStatus, tt.wantCode)

			msg := decode[errorBody](t, rec).Error.Message
			if tt.wantMsg != "" && !strings.Contains(msg, tt.wantMsg) {
				t.Errorf("message = %q, want it to contain %q", msg, tt.wantMsg)
			}
			if strings.Contains(msg, "db-secret-host") {
				t.Errorf("internal error details leaked to the client: %q", msg)
			}
		})
	}
}

func TestShortURLOrigin(t *testing.T) {
	svc := &fakeService{shorten: func(shortener.ShortenInput) (store.Link, error) {
		return store.Link{Code: "aB3dE9x", URL: "https://example.com", CreatedAt: t0}, nil
	}}

	tests := []struct {
		name    string
		opts    []Option
		tls     bool
		host    string
		wantURL string
	}{
		{"configured base URL wins", []Option{WithBaseURL("https://sho.example.com")}, false, "internal:8080", "https://sho.example.com/aB3dE9x"},
		{"trailing slash in base URL is dropped", []Option{WithBaseURL("https://sho.example.com///")}, false, "internal:8080", "https://sho.example.com/aB3dE9x"},
		{"derived from plain HTTP request", nil, false, "localhost:8080", "http://localhost:8080/aB3dE9x"},
		{"derived from TLS request", nil, true, "sho.example.com", "https://sho.example.com/aB3dE9x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"https://example.com"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Host = tt.host
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			rec := httptest.NewRecorder()
			newMux(svc, tt.opts...).ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201", rec.Code)
			}
			if got := decode[linkResponse](t, rec).ShortURL; got != tt.wantURL {
				t.Errorf("short_url = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestRedirect(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		resolve    func(string) (store.Link, error)
		wantStatus int
		wantCode   string
		wantTarget string
		wantCalls  int
	}{
		{
			name: "existing link",
			path: "/aB3dE9x",
			resolve: func(code string) (store.Link, error) {
				return store.Link{Code: code, URL: "https://example.com/landing?a=1&b=2"}, nil
			},
			wantStatus: http.StatusFound,
			wantTarget: "https://example.com/landing?a=1&b=2",
			wantCalls:  1,
		},
		{
			name:       "unknown code",
			path:       "/zzzzzzz",
			resolve:    func(string) (store.Link, error) { return store.Link{}, shortener.ErrNotFound },
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantCalls:  1,
		},
		{
			name:       "expired link",
			path:       "/aB3dE9x",
			resolve:    func(string) (store.Link, error) { return store.Link{}, shortener.ErrExpired },
			wantStatus: http.StatusGone,
			wantCode:   "link_expired",
			wantCalls:  1,
		},
		{
			name:       "unexpected failure",
			path:       "/aB3dE9x",
			resolve:    func(string) (store.Link, error) { return store.Link{}, errors.New("boom") },
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantCalls:  1,
		},
		{name: "code too short", path: "/abc", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "code too long", path: "/abcdefghijk", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "file-like path", path: "/favicon.ico", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "invalid characters", path: "/ab-cd_ef", wantStatus: http.StatusNotFound, wantCode: "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{resolve: tt.resolve}
			rec := do(t, newMux(svc), http.MethodGet, tt.path, "", "")

			if svc.calls != tt.wantCalls {
				t.Errorf("service called %d times, want %d", svc.calls, tt.wantCalls)
			}
			if tt.wantStatus == http.StatusFound {
				if rec.Code != http.StatusFound {
					t.Fatalf("status = %d, want 302", rec.Code)
				}
				if loc := rec.Header().Get("Location"); loc != tt.wantTarget {
					t.Errorf("Location = %q, want %q", loc, tt.wantTarget)
				}
				if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
					t.Errorf("Cache-Control = %q, want no-store", cc)
				}
				return
			}
			assertError(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}

func TestLinkStats(t *testing.T) {
	last := t0.Add(30 * time.Minute)
	full := shortener.Stats{
		Link: store.Link{
			Code: "aB3dE9x", URL: "https://example.com", CreatedAt: t0,
			ExpiresAt: t0.Add(time.Hour), Hits: 7, LastAccessAt: last,
		},
		Expired: true,
	}
	minimal := shortener.Stats{Link: store.Link{Code: "aB3dE9x", URL: "https://example.com", CreatedAt: t0}}

	t.Run("complete stats", func(t *testing.T) {
		svc := &fakeService{stats: func(string) (shortener.Stats, error) { return full, nil }}
		rec := do(t, newMux(svc), http.MethodGet, "/api/links/aB3dE9x/stats", "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}
		got := decode[statsResponse](t, rec)
		if got.Code != "aB3dE9x" || got.URL != "https://example.com" || got.Hits != 7 || !got.Expired {
			t.Errorf("unexpected body: %+v", got)
		}
		if got.ExpiresAt == nil || !got.ExpiresAt.Equal(full.ExpiresAt) {
			t.Errorf("expires_at = %v, want %v", got.ExpiresAt, full.ExpiresAt)
		}
		if got.LastAccessAt == nil || !got.LastAccessAt.Equal(last) {
			t.Errorf("last_access_at = %v, want %v", got.LastAccessAt, last)
		}
	})

	t.Run("optional fields are omitted", func(t *testing.T) {
		svc := &fakeService{stats: func(string) (shortener.Stats, error) { return minimal, nil }}
		rec := do(t, newMux(svc), http.MethodGet, "/api/links/aB3dE9x/stats", "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		for _, field := range []string{"expires_at", "last_access_at"} {
			if strings.Contains(rec.Body.String(), field) {
				t.Errorf("%s should be omitted: %s", field, rec.Body.String())
			}
		}
		if got := decode[statsResponse](t, rec); got.Hits != 0 || got.Expired {
			t.Errorf("unexpected body: %+v", got)
		}
	})

	errorCases := []struct {
		name       string
		path       string
		err        error
		wantStatus int
		wantCode   string
		wantCalls  int
	}{
		{"unknown code", "/api/links/zzzzzzz/stats", shortener.ErrNotFound, 404, "not_found", 1},
		{"unexpected failure", "/api/links/aB3dE9x/stats", errors.New("boom"), 500, "internal_error", 1},
		{"malformed code", "/api/links/not.a.code/stats", nil, 404, "not_found", 0},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{stats: func(string) (shortener.Stats, error) { return shortener.Stats{}, tt.err }}
			rec := do(t, newMux(svc), http.MethodGet, tt.path, "", "")
			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if svc.calls != tt.wantCalls {
				t.Errorf("service called %d times, want %d", svc.calls, tt.wantCalls)
			}
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	tests := []struct {
		method string
		path   string
		allow  string
	}{
		{http.MethodGet, "/api/links", "POST"},
		{http.MethodDelete, "/api/links", "POST"},
		{http.MethodPost, "/aB3dE9x", "GET"},
		{http.MethodPut, "/api/links/aB3dE9x/stats", "GET"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := do(t, newMux(&fakeService{}), tt.method, tt.path, "", "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, tt.allow) {
				t.Errorf("Allow = %q, want it to contain %q", allow, tt.allow)
			}
		})
	}
}

func TestUnexpectedErrorsAreLogged(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	svc := &fakeService{resolve: func(string) (store.Link, error) { return store.Link{}, errors.New("disk exploded") }}

	rec := do(t, newMux(svc, WithLogger(logger)), http.MethodGet, "/aB3dE9x", "", "")
	assertError(t, rec, http.StatusInternalServerError, "internal_error")

	if !strings.Contains(logs.String(), "disk exploded") {
		t.Errorf("underlying error was not logged: %q", logs.String())
	}
}
