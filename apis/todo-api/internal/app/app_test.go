package app

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/config"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

func testConfig() config.Config {
	return config.Config{
		Port: 8080, LogLevel: "info", CORSOrigins: []string{"https://app.example.com"},
		RateLimitRPS: 1000, RateLimitBurst: 1000,
	}
}

func newHandler(t *testing.T, cfg config.Config) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	return NewHandler(cfg, todo.NewMemoryStore(), logger), &logs
}

func send(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.RemoteAddr = "203.0.113.5:4000"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequestsGetAnIDAndAreLogged(t *testing.T) {
	h, logs := newHandler(t, testConfig())

	rec := send(h, http.MethodGet, "/healthz", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("X-Request-ID response header is missing")
	}
	if !strings.Contains(logs.String(), id) {
		t.Errorf("the request id %q should appear in the log line: %s", id, logs.String())
	}
}

func TestWritesRequireAnAPIKeyWhenConfigured(t *testing.T) {
	cfg := testConfig()
	cfg.APIKeys = []string{"s3cret"}
	h, _ := newHandler(t, cfg)
	const body = `{"title":"guarded"}`

	if got := send(h, http.MethodPost, "/api/todos", body, nil).Code; got != http.StatusUnauthorized {
		t.Errorf("POST without key status = %d, want 401", got)
	}
	if got := send(h, http.MethodPost, "/api/todos", body, map[string]string{"X-API-Key": "s3cret"}).Code; got != http.StatusCreated {
		t.Errorf("POST with key status = %d, want 201", got)
	}
	if got := send(h, http.MethodGet, "/api/todos", "", nil).Code; got != http.StatusOK {
		t.Errorf("GET without key status = %d, want 200 because reads are public", got)
	}
}

func TestWritesAreOpenWithoutConfiguredKeys(t *testing.T) {
	h, _ := newHandler(t, testConfig())

	if got := send(h, http.MethodPost, "/api/todos", `{"title":"open"}`, nil).Code; got != http.StatusCreated {
		t.Errorf("POST status = %d, want 201 when no key is configured", got)
	}
}

func TestCORSHeadersAndPreflight(t *testing.T) {
	cfg := testConfig()
	cfg.APIKeys = []string{"s3cret"}
	h, _ := newHandler(t, cfg)
	origin := map[string]string{"Origin": "https://app.example.com"}

	rec := send(h, http.MethodGet, "/api/todos", "", origin)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Allow-Origin = %q, want the caller's origin", got)
	}

	// A browser preflight carries no credentials, so it must be answered
	// before the API key check or no browser could ever write.
	preflight := send(h, http.MethodOptions, "/api/todos", "", map[string]string{
		"Origin":                         "https://app.example.com",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "content-type,x-api-key",
	})
	if preflight.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", preflight.Code)
	}
}

func TestRateLimitApplies(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimitRPS = 1
	cfg.RateLimitBurst = 2
	h, _ := newHandler(t, cfg)

	for i := 0; i < 2; i++ {
		if got := send(h, http.MethodGet, "/healthz", "", nil).Code; got != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i+1, got)
		}
	}
	rec := send(h, http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("third request status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("even throttled responses must carry a request id")
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	h, _ := newHandler(t, testConfig())

	if got := send(h, http.MethodGet, "/nope", "", nil).Code; got != http.StatusNotFound {
		t.Errorf("unknown path status = %d, want 404", got)
	}
	if got := send(h, http.MethodPut, "/api/todos", `{"title":"x"}`, nil).Code; got != http.StatusMethodNotAllowed {
		t.Errorf("unsupported method status = %d, want 405", got)
	}
}
