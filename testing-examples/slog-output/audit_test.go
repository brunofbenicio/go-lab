package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"testing/slogtest"
)

// dropTime removes the timestamp, the only non-deterministic attribute, so
// the output can be compared exactly.
func dropTime(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}

// newLogger returns a logger writing JSON lines into a buffer the test can
// inspect. wrap lets a test insert a handler (such as the redactor).
func newLogger(level slog.Level, wrap func(slog.Handler) slog.Handler) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	var h slog.Handler = slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level, ReplaceAttr: dropTime})
	if wrap != nil {
		h = wrap(h)
	}
	return slog.New(h), &buf
}

// records parses the JSON lines in buf. Comparing parsed maps is more robust
// than comparing raw text: key order and spacing do not matter.
func records(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid JSON log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestLoginSuccessLogsInfoOnly(t *testing.T) {
	logger, buf := newLogger(slog.LevelInfo, nil)
	svc := NewService(logger, func(string, string) bool { return true })

	if err := svc.Login(t.Context(), "ana", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	want := []map[string]any{
		{"level": "INFO", "msg": "login ok", "component": "auth", "user": "ana"},
	}
	// The Debug line "session created" is filtered by the handler level.
	if got := records(t, buf); !reflect.DeepEqual(got, want) {
		t.Errorf("log = %v\nwant %v", got, want)
	}
}

func TestDebugLevelShowsMoreRecords(t *testing.T) {
	logger, buf := newLogger(slog.LevelDebug, nil)
	svc := NewService(logger, func(string, string) bool { return true })
	if err := svc.Login(t.Context(), "ana", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	got := records(t, buf)
	if len(got) != 2 || got[1]["msg"] != "session created" || got[1]["level"] != "DEBUG" {
		t.Errorf("log = %v, want the info line followed by a debug line", got)
	}
}

func TestLoginFailureRedactsPassword(t *testing.T) {
	logger, buf := newLogger(slog.LevelInfo, func(h slog.Handler) slog.Handler {
		return NewRedactHandler(h, "password")
	})
	svc := NewService(logger, func(string, string) bool { return false })

	err := svc.Login(t.Context(), "ana", "hunter2")
	if !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("Login error = %v, want ErrBadCredentials", err)
	}

	// Strongest check first: the secret must not appear anywhere in the output.
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("password leaked into the log: %s", buf.String())
	}
	want := []map[string]any{{
		"level": "WARN", "msg": "login failed", "component": "auth", "user": "ana",
		"request": map[string]any{"password": "[REDACTED]", "ip": "192.0.2.10"},
	}}
	if got := records(t, buf); !reflect.DeepEqual(got, want) {
		t.Errorf("log = %v\nwant %v", got, want)
	}
}

func TestRedactAlsoCoversWithAttrs(t *testing.T) {
	logger, buf := newLogger(slog.LevelInfo, func(h slog.Handler) slog.Handler {
		return NewRedactHandler(h, "token")
	})
	logger.With("token", "abc123").Info("call", "id", 7)

	if strings.Contains(buf.String(), "abc123") {
		t.Errorf("token leaked: %s", buf.String())
	}
	if got := records(t, buf); got[0]["token"] != "[REDACTED]" || got[0]["id"] != float64(7) {
		t.Errorf("log = %v", got)
	}
}

// Text output can be asserted line by line when the format is simple.
func TestTextHandlerOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{ReplaceAttr: dropTime}))

	logger.Warn("disk almost full", "used_pct", 93)

	if got, want := buf.String(), "level=WARN msg=\"disk almost full\" used_pct=93\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// slogtest.TestHandler runs the standard conformance suite against a custom
// Handler: groups, WithAttrs, empty attributes, zero time and so on.
func TestRedactHandlerConformance(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactHandler(slog.NewJSONHandler(&buf, nil), "password")

	err := slogtest.TestHandler(h, func() []map[string]any { return records(t, &buf) })
	if err != nil {
		t.Errorf("handler does not conform to slog semantics:\n%v", err)
	}
}

// Compile-time check that RedactHandler implements slog.Handler.
var _ slog.Handler = (*RedactHandler)(nil)

func TestEnabledDelegates(t *testing.T) {
	logger, _ := newLogger(slog.LevelWarn, func(h slog.Handler) slog.Handler { return NewRedactHandler(h) })
	if logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Info should be disabled when the wrapped handler is at Warn")
	}
	if !logger.Enabled(context.Background(), slog.LevelError) {
		t.Error("Error should be enabled")
	}
}
