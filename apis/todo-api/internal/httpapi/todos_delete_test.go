package httpapi

import (
	"net/http"
	"testing"
)

func TestDeleteTodo(t *testing.T) {
	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"temporary"}`)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"keeper"}`)

	rec := do(t, h, http.MethodDelete, "/api/todos/1", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want it empty for 204", rec.Body.String())
	}
	if got := do(t, h, http.MethodGet, "/api/todos/1", "").Code; got != http.StatusNotFound {
		t.Errorf("GET after delete status = %d, want %d", got, http.StatusNotFound)
	}
	if got := do(t, h, http.MethodGet, "/api/todos/2", "").Code; got != http.StatusOK {
		t.Errorf("the other todo must survive, GET status = %d", got)
	}
}

func TestDeleteTodoErrors(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantCode   string
	}{
		{name: "missing todo", target: "/api/todos/5", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "invalid id", target: "/api/todos/nope", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
	}

	h := newTestRouter(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodDelete, tt.target, "")

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

func TestDeleteTwiceReturnsNotFound(t *testing.T) {
	h := newTestRouter(t)
	do(t, h, http.MethodPost, "/api/todos", `{"title":"once"}`)

	first := do(t, h, http.MethodDelete, "/api/todos/1", "")
	second := do(t, h, http.MethodDelete, "/api/todos/1", "")

	if first.Code != http.StatusNoContent || second.Code != http.StatusNotFound {
		t.Errorf("statuses = %d then %d, want 204 then 404", first.Code, second.Code)
	}
}
