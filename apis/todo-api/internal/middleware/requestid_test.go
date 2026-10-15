package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestRequestID(t *testing.T) {
	generated := regexp.MustCompile(`^[0-9a-f]{32}$`)

	tests := []struct {
		name       string
		incoming   string
		wantExact  string
		wantRandom bool
	}{
		{name: "generates when missing", incoming: "", wantRandom: true},
		{name: "keeps a valid client id", incoming: "trace-123_abc.v2", wantExact: "trace-123_abc.v2"},
		{name: "replaces ids with unsafe characters", incoming: "bad id\r\nX: y", wantRandom: true},
		{name: "replaces ids that are too long", incoming: strings.Repeat("a", 65), wantRandom: true},
		{name: "accepts the longest allowed id", incoming: strings.Repeat("a", 64), wantExact: strings.Repeat("a", 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			h := RequestID()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = RequestIDFrom(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.incoming != "" {
				req.Header.Set(RequestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			got := rec.Header().Get(RequestIDHeader)
			if tt.wantRandom && !generated.MatchString(got) {
				t.Errorf("response id = %q, want 32 hex characters", got)
			}
			if !tt.wantRandom && got != tt.wantExact {
				t.Errorf("response id = %q, want %q", got, tt.wantExact)
			}
			if seen != got {
				t.Errorf("context id = %q, want the same as the header %q", seen, got)
			}
		})
	}
}

func TestRequestIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	h := RequestID()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for i := 0; i < 50; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		id := rec.Header().Get(RequestIDHeader)
		if seen[id] {
			t.Fatalf("duplicate request id %q", id)
		}
		seen[id] = true
	}
}

func TestRequestIDFromEmptyContext(t *testing.T) {
	if got := RequestIDFrom(httptest.NewRequest(http.MethodGet, "/", nil).Context()); got != "" {
		t.Errorf("RequestIDFrom() = %q, want empty", got)
	}
}

func TestLoggingIncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := Chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		RequestID(),
		Logging(newLogger(&buf)),
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "abc-1")

	h.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if entry["request_id"] != "abc-1" {
		t.Errorf("request_id = %v, want abc-1", entry["request_id"])
	}
}
