package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/config"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// e2e drives the whole stack through a real HTTP server and client, so the
// tests exercise sockets, headers and serialization exactly as a consumer would.
type e2e struct {
	t      *testing.T
	server *httptest.Server
	key    string
}

func newE2E(t *testing.T) *e2e {
	t.Helper()
	cfg := config.Config{
		Port: 8080, LogLevel: "info",
		APIKeys:      []string{"e2e-key"},
		CORSOrigins:  []string{"*"},
		RateLimitRPS: 10000, RateLimitBurst: 10000,
	}
	handler := NewHandler(cfg, todo.NewMemoryStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &e2e{t: t, server: srv, key: "e2e-key"}
}

// call sends a request and returns the status, headers and raw body.
func (e *e2e) call(method, path, body string, withKey bool) (int, http.Header, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if withKey {
		req.Header.Set("X-API-Key", e.key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, raw
}

func (e *e2e) mustStatus(want int, method, path, body string, withKey bool) []byte {
	e.t.Helper()
	status, _, raw := e.call(method, path, body, withKey)
	if status != want {
		e.t.Fatalf("%s %s = %d, want %d; body: %s", method, path, status, want, raw)
	}
	return raw
}

func unmarshal[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return v
}

type page struct {
	Items  []todo.Todo `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func TestTodoLifecycle(t *testing.T) {
	e := newE2E(t)

	// Create two todos; one is only valid with a key.
	e.mustStatus(http.StatusUnauthorized, http.MethodPost, "/api/todos", `{"title":"no key"}`, false)
	created := unmarshal[todo.Todo](t, e.mustStatus(http.StatusCreated, http.MethodPost, "/api/todos",
		`{"title":"Write the e2e test","notes":"cover the full lifecycle","due_date":"2026-12-24T10:00:00Z"}`, true))
	if created.ID != 1 || created.Done {
		t.Fatalf("created = %+v, want id 1 and not done", created)
	}
	e.mustStatus(http.StatusCreated, http.MethodPost, "/api/todos", `{"title":"Water the plants"}`, true)

	// Read it back, publicly.
	got := unmarshal[todo.Todo](t, e.mustStatus(http.StatusOK, http.MethodGet, "/api/todos/1", "", false))
	if got.Title != "Write the e2e test" || got.DueDate == nil {
		t.Errorf("got %+v, want the created todo with its due date", got)
	}

	// Complete it and search.
	patched := unmarshal[todo.Todo](t, e.mustStatus(http.StatusOK, http.MethodPatch, "/api/todos/1", `{"done":true}`, true))
	if !patched.Done || patched.Notes != "cover the full lifecycle" {
		t.Errorf("patched = %+v, want done with notes untouched", patched)
	}
	done := unmarshal[page](t, e.mustStatus(http.StatusOK, http.MethodGet, "/api/todos?done=true", "", false))
	if done.Total != 1 || done.Items[0].ID != 1 {
		t.Errorf("done list = %+v, want only todo 1", done)
	}
	found := unmarshal[page](t, e.mustStatus(http.StatusOK, http.MethodGet, "/api/todos?q=PLANTS", "", false))
	if found.Total != 1 || found.Items[0].ID != 2 {
		t.Errorf("search = %+v, want only todo 2", found)
	}

	// Replace wipes what the body omits.
	replaced := unmarshal[todo.Todo](t, e.mustStatus(http.StatusOK, http.MethodPut, "/api/todos/1", `{"title":"Rewritten"}`, true))
	if replaced.Done || replaced.Notes != "" || replaced.DueDate != nil {
		t.Errorf("replaced = %+v, want every omitted field reset", replaced)
	}

	// Delete and confirm it is gone while the other survives.
	e.mustStatus(http.StatusNoContent, http.MethodDelete, "/api/todos/1", "", true)
	e.mustStatus(http.StatusNotFound, http.MethodGet, "/api/todos/1", "", false)
	e.mustStatus(http.StatusNotFound, http.MethodDelete, "/api/todos/1", "", true)
	rest := unmarshal[page](t, e.mustStatus(http.StatusOK, http.MethodGet, "/api/todos", "", false))
	if rest.Total != 1 || rest.Items[0].ID != 2 {
		t.Errorf("after delete = %+v, want only todo 2", rest)
	}
}

func TestPaginationOverHTTP(t *testing.T) {
	e := newE2E(t)
	for i := 1; i <= 7; i++ {
		e.mustStatus(http.StatusCreated, http.MethodPost, "/api/todos", fmt.Sprintf(`{"title":"item %d"}`, i), true)
	}

	var seen []int64
	for offset := 0; offset < 7; offset += 3 {
		status, headers, raw := e.call(http.MethodGet, fmt.Sprintf("/api/todos?limit=3&offset=%d", offset), "", false)
		if status != http.StatusOK {
			t.Fatalf("page at offset %d status = %d", offset, status)
		}
		if headers.Get("X-Total-Count") != "7" {
			t.Errorf("X-Total-Count = %q, want 7", headers.Get("X-Total-Count"))
		}
		for _, it := range unmarshal[page](t, raw).Items {
			seen = append(seen, it.ID)
		}
	}

	if fmt.Sprint(seen) != "[1 2 3 4 5 6 7]" {
		t.Errorf("walking the pages visited %v, want every todo once and in order", seen)
	}
}

func TestErrorEnvelopeOverHTTP(t *testing.T) {
	e := newE2E(t)

	status, headers, raw := e.call(http.MethodPost, "/api/todos", `{"title":""}`, true)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", status)
	}
	if headers.Get("Content-Type") != "application/json" || headers.Get("X-Request-ID") == "" {
		t.Errorf("headers = %v, want JSON content type and a request id", headers)
	}
	body := unmarshal[struct {
		Error struct {
			Code    string `json:"code"`
			Details []struct {
				Field string `json:"field"`
			} `json:"details"`
		} `json:"error"`
	}](t, raw)
	if body.Error.Code != "validation_failed" || len(body.Error.Details) != 1 || body.Error.Details[0].Field != "title" {
		t.Errorf("error = %+v, want validation_failed pointing at title", body.Error)
	}
}

func TestDocumentationOverHTTP(t *testing.T) {
	e := newE2E(t)

	spec := e.mustStatus(http.StatusOK, http.MethodGet, "/openapi.yaml", "", false)
	if !bytes.HasPrefix(spec, []byte("openapi: 3.0.3")) {
		t.Errorf("spec starts with %.20q, want the OpenAPI header", spec)
	}
	ui := e.mustStatus(http.StatusOK, http.MethodGet, "/docs", "", false)
	if !bytes.Contains(ui, []byte("swagger-ui")) {
		t.Error("/docs should serve the Swagger UI page")
	}
}

func TestConcurrentClients(t *testing.T) {
	e := newE2E(t)
	const clients = 25

	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _, raw := e.call(http.MethodPost, "/api/todos", `{"title":"parallel"}`, true)
			if status != http.StatusCreated {
				t.Errorf("POST status = %d, body: %s", status, raw)
			}
		}()
	}
	wg.Wait()

	all := unmarshal[page](t, e.mustStatus(http.StatusOK, http.MethodGet, "/api/todos?limit=200", "", false))
	if all.Total != clients {
		t.Fatalf("total = %d, want %d", all.Total, clients)
	}
	ids := make(map[int64]bool)
	for _, it := range all.Items {
		ids[it.ID] = true
	}
	if len(ids) != clients {
		t.Errorf("got %d distinct ids, want %d: concurrent creates must never share an id", len(ids), clients)
	}
}
