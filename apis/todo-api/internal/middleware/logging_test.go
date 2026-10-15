package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestLogging(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantLevel  string
		wantBytes  int
	}{
		{
			name:       "implicit 200 is logged at info",
			handler:    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) },
			wantStatus: 200, wantLevel: "INFO", wantBytes: 5,
		},
		{
			name:       "client error is logged at warn",
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			wantStatus: 404, wantLevel: "WARN",
		},
		{
			name:       "server error is logged at error",
			handler:    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusInternalServerError) },
			wantStatus: 500, wantLevel: "ERROR", wantBytes: len("boom\n"),
		},
		{
			name:       "handler that writes nothing counts as 200",
			handler:    func(http.ResponseWriter, *http.Request) {},
			wantStatus: 200, wantLevel: "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := Logging(newLogger(&buf))(tt.handler)
			req := httptest.NewRequest(http.MethodGet, "/api/todos?x=1", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
			}
			if entry["level"] != tt.wantLevel {
				t.Errorf("level = %v, want %s", entry["level"], tt.wantLevel)
			}
			if int(entry["status"].(float64)) != tt.wantStatus {
				t.Errorf("status = %v, want %d", entry["status"], tt.wantStatus)
			}
			if int(entry["bytes"].(float64)) != tt.wantBytes {
				t.Errorf("bytes = %v, want %d", entry["bytes"], tt.wantBytes)
			}
			if entry["method"] != "GET" || entry["path"] != "/api/todos" {
				t.Errorf("method/path = %v %v, want GET /api/todos (query string must not be logged)", entry["method"], entry["path"])
			}
			if _, ok := entry["duration"]; !ok {
				t.Error("duration attribute is missing")
			}
		})
	}
}

func TestLoggingLogsOncePerRequest(t *testing.T) {
	var buf bytes.Buffer
	h := Logging(newLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))

	if lines := strings.Count(strings.TrimSpace(buf.String()), "\n") + 1; lines != 1 {
		t.Errorf("got %d log lines, want exactly 1", lines)
	}
}

func TestChainOrder(t *testing.T) {
	var trace []string
	tag := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trace = append(trace, name+":in")
				next.ServeHTTP(w, r)
				trace = append(trace, name+":out")
			})
		}
	}
	final := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { trace = append(trace, "handler") })

	Chain(final, tag("a"), tag("b")).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := "a:in b:in handler b:out a:out"
	if got := strings.Join(trace, " "); got != want {
		t.Errorf("execution order = %q, want %q", got, want)
	}
}
