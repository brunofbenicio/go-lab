package todoapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// do sends one request straight into the handler. There is no listener, no
// port and no network: httptest.NewRecorder is an in-memory ResponseWriter
// that remembers the status, headers and body the handler wrote.
func do(t *testing.T, h http.Handler, method, target, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result() // the *http.Response a real client would have seen
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return v
}

func TestCreateAndFetch(t *testing.T) {
	h := NewServer().Handler()

	resp := do(t, h, "POST", "/todos", `{"title":"write tests"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/todos/1" {
		t.Errorf("Location = %q, want /todos/1", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	created := decode[Todo](t, resp)
	if want := (Todo{ID: 1, Title: "write tests"}); created != want {
		t.Errorf("created = %+v, want %+v", created, want)
	}

	resp = do(t, h, "GET", resp.Header.Get("Location"), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}
	if got := decode[Todo](t, resp); got != created {
		t.Errorf("fetched = %+v, want %+v", got, created)
	}
}

func TestListIsSortedAndNeverNull(t *testing.T) {
	h := NewServer().Handler()

	// An empty list must be [] and not null: clients iterate over it.
	resp := do(t, h, "GET", "/todos", "")
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != "[]" {
		t.Errorf("empty list body = %q, want []", got)
	}

	for _, title := range []string{"a", "b", "c"} {
		do(t, h, "POST", "/todos", `{"title":"`+title+`"}`)
	}
	todos := decode[[]Todo](t, do(t, h, "GET", "/todos", ""))
	if len(todos) != 3 || todos[0].ID != 1 || todos[2].Title != "c" {
		t.Errorf("list = %+v, want 3 todos sorted by id", todos)
	}
}

func TestDelete(t *testing.T) {
	h := NewServer().Handler()
	do(t, h, "POST", "/todos", `{"title":"temp"}`)

	if got := do(t, h, "DELETE", "/todos/1", "").StatusCode; got != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", got)
	}
	if got := do(t, h, "GET", "/todos/1", "").StatusCode; got != http.StatusNotFound {
		t.Errorf("GET after DELETE status = %d, want 404", got)
	}
	if got := do(t, h, "DELETE", "/todos/1", "").StatusCode; got != http.StatusNotFound {
		t.Errorf("second DELETE status = %d, want 404", got)
	}
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
		want   int
	}{
		{"malformed JSON", "POST", "/todos", `{"title":`, http.StatusBadRequest},
		{"missing title", "POST", "/todos", `{"title":"  "}`, http.StatusUnprocessableEntity},
		{"id is not a number", "GET", "/todos/abc", "", http.StatusBadRequest},
		{"unknown id", "GET", "/todos/99", "", http.StatusNotFound},
		{"method not allowed", "PUT", "/todos/1", "", http.StatusMethodNotAllowed},
		{"unknown route", "GET", "/nothing", "", http.StatusNotFound},
	}
	h := NewServer().Handler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(t, h, tt.method, tt.target, tt.body).StatusCode; got != tt.want {
				t.Errorf("%s %s status = %d, want %d", tt.method, tt.target, got, tt.want)
			}
		})
	}
}

func TestCreateRequiresJSONContentType(t *testing.T) {
	req := httptest.NewRequest("POST", "/todos", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	NewServer().Handler().ServeHTTP(rec, req)

	// The recorder also exposes Code and Body directly, without Result().
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "application/json") {
		t.Errorf("body = %q, want a hint about application/json", rec.Body.String())
	}
}

// A single handler method can be tested without the router, but then the
// path wildcard must be provided by hand with SetPathValue.
func TestHandlerMethodWithoutRouter(t *testing.T) {
	s := NewServer()
	s.todos[7] = Todo{ID: 7, Title: "direct"}

	req := httptest.NewRequest("GET", "/todos/7", nil)
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()
	s.get(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"direct"`) {
		t.Errorf("got %d %q, want 200 with the todo", rec.Code, rec.Body.String())
	}
}
