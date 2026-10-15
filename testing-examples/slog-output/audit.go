// Package audit shows how to test code that logs with log/slog. It contains a
// small service that logs its decisions and a handler wrapper that redacts
// sensitive attributes before they reach the log output.
package audit

import (
	"context"
	"errors"
	"log/slog"
)

// ErrBadCredentials is returned by Login when authentication fails.
var ErrBadCredentials = errors.New("audit: bad credentials")

// Service authenticates users and logs the outcome.
type Service struct {
	log   *slog.Logger
	check func(user, password string) bool
}

// NewService returns a Service that logs to l and validates with check.
func NewService(l *slog.Logger, check func(user, password string) bool) *Service {
	return &Service{log: l.With("component", "auth"), check: check}
}

// Login authenticates the user and logs the outcome. On failure it
// (deliberately) includes the submitted password in the log record; the
// RedactHandler is the safety net that keeps it out of the output.
func (s *Service) Login(ctx context.Context, user, password string) error {
	if !s.check(user, password) {
		s.log.WarnContext(ctx, "login failed",
			slog.String("user", user),
			slog.Group("request", slog.String("password", password), slog.String("ip", "192.0.2.10")))
		return ErrBadCredentials
	}
	s.log.InfoContext(ctx, "login ok", "user", user)
	s.log.Debug("session created", "user", user)
	return nil
}

// RedactHandler wraps another slog.Handler and replaces the value of the given
// keys with "[REDACTED]", including inside groups.
type RedactHandler struct {
	next slog.Handler
	keys map[string]bool
}

// NewRedactHandler wraps next, masking attributes whose key is in keys.
func NewRedactHandler(next slog.Handler, keys ...string) *RedactHandler {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return &RedactHandler{next: next, keys: set}
}

// Enabled implements slog.Handler.
func (h *RedactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

// Handle implements slog.Handler.
func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(h.redact(a))
		return true
	})
	return h.next.Handle(ctx, clean)
}

// WithAttrs implements slog.Handler.
func (h *RedactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	masked := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		masked[i] = h.redact(a)
	}
	return &RedactHandler{next: h.next.WithAttrs(masked), keys: h.keys}
}

// WithGroup implements slog.Handler.
func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{next: h.next.WithGroup(name), keys: h.keys}
}

func (h *RedactHandler) redact(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve() // expand LogValuer implementations first
	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		masked := make([]slog.Attr, len(group))
		for i, ga := range group {
			masked[i] = h.redact(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(masked...)}
	}
	if h.keys[a.Key] {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}
