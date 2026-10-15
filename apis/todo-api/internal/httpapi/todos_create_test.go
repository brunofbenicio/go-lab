package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

func TestCreateTodo(t *testing.T) {
	h := newTestRouter(t)

	rec := do(t, h, http.MethodPost, "/api/todos", `{"title":"  Buy milk  ","notes":"2 liters"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/api/todos/1" {
		t.Errorf("Location = %q, want /api/todos/1", got)
	}
	var created todo.Todo
	decode(t, rec, &created)
	if created.ID != 1 || created.Title != "Buy milk" || created.Notes != "2 liters" || created.Done {
		t.Errorf("created = %+v, want id 1, trimmed title, notes and done=false", created)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created_at must be set")
	}

	second := do(t, h, http.MethodPost, "/api/todos", `{"title":"Second"}`)
	if got := second.Header().Get("Location"); got != "/api/todos/2" {
		t.Errorf("second Location = %q, want /api/todos/2", got)
	}
}

func TestCreateTodoErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "empty body", body: "", wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "malformed json", body: `{"title":`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "syntax error", body: `{title: "x"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "wrong field type", body: `{"title": 42}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "unknown field", body: `{"title":"x","color":"red"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "trailing data", body: `{"title":"x"}{"title":"y"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "invalid due date", body: `{"title":"x","due_date":"tomorrow"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "blank title", body: `{"title":"   "}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "title too long", body: `{"title":"` + strings.Repeat("a", todo.MaxTitleLength+1) + `"}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "body too large", body: `{"title":"x","notes":"` + strings.Repeat("n", maxBodyBytes) + `"}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "payload_too_large"},
	}

	h := newTestRouter(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/api/todos", tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %.200s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var body errorBody
			decode(t, rec, &body)
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestCreateTodoValidationDetails(t *testing.T) {
	h := newTestRouter(t)

	rec := do(t, h, http.MethodPost, "/api/todos", `{"title":"","notes":"`+strings.Repeat("n", todo.MaxNotesLength+1)+`"}`)

	var body errorBody
	decode(t, rec, &body)
	if len(body.Error.Details) != 2 {
		t.Fatalf("details = %+v, want one per invalid field", body.Error.Details)
	}
	if body.Error.Details[0].Field != "title" || body.Error.Details[1].Field != "notes" {
		t.Errorf("detail fields = %q, %q, want title, notes", body.Error.Details[0].Field, body.Error.Details[1].Field)
	}
}

func TestCreateTodoRejectsNonJSONContentType(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/todos", strings.NewReader(`title=x`))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}
