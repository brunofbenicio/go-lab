package httpapi

import (
	"net/http"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

func TestPatchTodo(t *testing.T) {
	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"write docs","notes":"readme","due_date":"2026-12-01T09:00:00Z"}`)

	t.Run("marks as done and keeps the rest", func(t *testing.T) {
		rec := do(t, h, http.MethodPatch, "/api/todos/1", `{"done":true}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var got todo.Todo
		decode(t, rec, &got)
		if !got.Done || got.Title != "write docs" || got.Notes != "readme" || got.DueDate == nil {
			t.Errorf("got %+v, want only done changed", got)
		}
	})

	t.Run("null clears the due date", func(t *testing.T) {
		rec := do(t, h, http.MethodPatch, "/api/todos/1", `{"due_date":null}`)

		var got todo.Todo
		decode(t, rec, &got)
		if got.DueDate != nil {
			t.Errorf("due_date = %v, want it cleared", got.DueDate)
		}
	})

	t.Run("empty object is a no-op", func(t *testing.T) {
		rec := do(t, h, http.MethodPatch, "/api/todos/1", `{}`)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestPatchTodoErrors(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "missing todo", target: "/api/todos/9", body: `{"done":true}`, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "invalid id", target: "/api/todos/0", body: `{"done":true}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "blank title after merge", target: "/api/todos/1", body: `{"title":" "}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "unknown field", target: "/api/todos/1", body: `{"priority":1}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "wrong type", target: "/api/todos/1", body: `{"done":"yes"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
	}

	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"exists"}`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPatch, tt.target, tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var body errorBody
			decode(t, rec, &body)
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
