// Package middleware holds the cross-cutting HTTP concerns of the todo API:
// logging, request ids, panic recovery, CORS, rate limiting and authentication.
package middleware

import "net/http"

// Middleware decorates an http.Handler with extra behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with the given middlewares. The first one listed becomes the
// outermost layer, so Chain(h, a, b) runs a, then b, then h.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusWriter records the status code and body size written by a handler.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader records the status before delegating.
func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write counts the bytes and defaults the status to 200 like net/http does.
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// statusCode returns the recorded status, or 200 if the handler wrote nothing.
func (w *statusWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
