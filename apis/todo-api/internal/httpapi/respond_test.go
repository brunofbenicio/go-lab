package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, http.StatusCreated, map[string]int{"id": 7})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body["id"] != 7 {
		t.Errorf("body = %v, want id 7", body)
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		code    string
		message string
		details []fieldError
		want    errorBody
	}{
		{
			name:    "without details",
			status:  http.StatusNotFound,
			code:    "not_found",
			message: "todo 3 not found",
			want:    errorBody{Error: errorDetail{Code: "not_found", Message: "todo 3 not found"}},
		},
		{
			name:    "with field details",
			status:  http.StatusUnprocessableEntity,
			code:    "validation_failed",
			message: "request is invalid",
			details: []fieldError{{Field: "title", Message: "must not be empty"}},
			want: errorBody{Error: errorDetail{
				Code:    "validation_failed",
				Message: "request is invalid",
				Details: []fieldError{{Field: "title", Message: "must not be empty"}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeError(rec, tt.status, tt.code, tt.message, tt.details...)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			var got errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not valid JSON: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("body = %+v, want %+v", got, tt.want)
			}
		})
	}
}
