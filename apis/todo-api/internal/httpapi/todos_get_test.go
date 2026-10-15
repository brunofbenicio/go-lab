package httpapi

import (
	"net/http"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

func TestGetTodo(t *testing.T) {
	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"first"}`)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"second","done":true}`)

	rec := do(t, h, http.MethodGet, "/api/todos/2", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got todo.Todo
	decode(t, rec, &got)
	if got.ID != 2 || got.Title != "second" || !got.Done {
		t.Errorf("got %+v, want the second todo", got)
	}
}

func TestGetTodoErrors(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantCode   string
	}{
		{name: "missing todo", target: "/api/todos/42", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "not a number", target: "/api/todos/abc", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "zero", target: "/api/todos/0", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "negative", target: "/api/todos/-3", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "overflow", target: "/api/todos/99999999999999999999", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
	}

	h := newTestRouter(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, tt.target, "")

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
