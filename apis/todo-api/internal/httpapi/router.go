// Package httpapi exposes the todo API over HTTP.
package httpapi

import "net/http"

// NewRouter returns the HTTP handler with every route registered.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	return mux
}
