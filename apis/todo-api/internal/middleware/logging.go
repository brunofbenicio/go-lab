package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logging writes one structured log line per request with the method, path,
// status, response size and duration. Server errors are logged at error level,
// client errors at warn and everything else at info.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			status := sw.statusCode()
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", sw.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("remote_addr", r.RemoteAddr),
			}
			if id := RequestIDFrom(r.Context()); id != "" {
				attrs = append(attrs, slog.String("request_id", id))
			}
			logger.LogAttrs(r.Context(), level, "request completed", attrs...)
		})
	}
}
