package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverTurnsPanicInto500(t *testing.T) {
	var buf bytes.Buffer
	h := Chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("kaboom") }),
		RequestID(),
		Recover(newLogger(&buf)),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/todos", nil)
	req.Header.Set(RequestIDHeader, "req-9")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, rec.Body.String())
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error code = %q, want internal_error", body.Error.Code)
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Error("the panic value must not leak to the client")
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if entry["level"] != "ERROR" || entry["panic"] != "kaboom" || entry["request_id"] != "req-9" {
		t.Errorf("log entry = %v, want error level, the panic value and the request id", entry)
	}
	if stack, _ := entry["stack"].(string); !strings.Contains(stack, "recover_test.go") {
		t.Error("log entry should carry the stack trace of the panic")
	}
}

func TestRecoverPassesNormalRequestsThrough(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(newLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if buf.Len() != 0 {
		t.Errorf("nothing should be logged, got %s", buf.String())
	}
}

func TestRecoverDoesNotRewriteStartedResponses(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(newLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		panic("late failure")
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want the already-sent %d", rec.Code, http.StatusAccepted)
	}
	if rec.Body.String() != "partial" {
		t.Errorf("body = %q, want it untouched", rec.Body.String())
	}
	if !strings.Contains(buf.String(), "late failure") {
		t.Error("the panic must still be logged")
	}
}

func TestRecoverRepanicsErrAbortHandler(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(newLogger(&buf))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", got)
		}
		if buf.Len() != 0 {
			t.Errorf("an intentional abort must not be logged, got %s", buf.String())
		}
	}()

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
