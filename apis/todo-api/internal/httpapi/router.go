// Package httpapi exposes the todo API over HTTP.
package httpapi

import (
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// NewRouter returns the HTTP handler with every route registered, backed by
// the given store.
func NewRouter(store todo.Store) http.Handler {
	api := &API{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /api/todos", api.createTodo)
	mux.HandleFunc("GET /api/todos/{id}", api.getTodo)
	return mux
}
