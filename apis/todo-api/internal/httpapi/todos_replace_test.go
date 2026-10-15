package httpapi

import (
	"net/http"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

func TestReplaceTodo(t *testing.T) {
	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"old","notes":"to be dropped","done":false}`)

	rec := do(t, h, http.MethodPut, "/api/todos/1", `{"title":" new title ","done":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got todo.Todo
	decode(t, rec, &got)
	if got.ID != 1 || got.Title != "new title" || !got.Done {
		t.Errorf("got %+v, want id 1, title %q and done", got, "new title")
	}
	if got.Notes != "" {
		t.Errorf("notes = %q, want them cleared because PUT replaces the whole resource", got.Notes)
	}

	// The change must be visible to later reads.
	var fetched todo.Todo
	decode(t, do(t, h, http.MethodGet, "/api/todos/1", ""), &fetched)
	if fetched.Title != "new title" {
		t.Errorf("fetched title = %q, want %q", fetched.Title, "new title")
	}
}

func TestReplaceTodoErrors(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "missing todo", target: "/api/todos/9", body: `{"title":"x"}`, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "invalid id", target: "/api/todos/x", body: `{"title":"x"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "malformed json", target: "/api/todos/1", body: `{`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "blank title", target: "/api/todos/1", body: `{"title":""}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
	}

	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"exists"}`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPut, tt.target, tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body errorBody
			decode(t, rec, &body)
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
