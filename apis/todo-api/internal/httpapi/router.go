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
	mux.HandleFunc("GET /api/todos", api.listTodos)
	mux.HandleFunc("POST /api/todos", api.createTodo)
	mux.HandleFunc("GET /api/todos/{id}", api.getTodo)
	mux.HandleFunc("PUT /api/todos/{id}", api.replaceTodo)
	mux.HandleFunc("PATCH /api/todos/{id}", api.patchTodo)
	return mux
}
