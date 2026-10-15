// Package httpapi exposes the todo API over HTTP.
package httpapi

import (
	"net/http"

	todoapi "github.com/brunofbenicio/go-lab/apis/todo-api"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/docs"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// Route identifies one endpoint of the API.
type Route struct {
	Method string
	Path   string
}

// endpoint couples a route with its handler. The table below is the single
// source of truth for the API surface: NewRouter registers it and the
// OpenAPI consistency test reads it through Routes.
type endpoint struct {
	Route
	handle func(a *API, w http.ResponseWriter, r *http.Request)
}

var endpoints = []endpoint{
	{Route{http.MethodGet, "/healthz"}, func(_ *API, w http.ResponseWriter, r *http.Request) { handleHealth(w, r) }},
	{Route{http.MethodGet, "/api/todos"}, (*API).listTodos},
	{Route{http.MethodPost, "/api/todos"}, (*API).createTodo},
	{Route{http.MethodGet, "/api/todos/{id}"}, (*API).getTodo},
	{Route{http.MethodPut, "/api/todos/{id}"}, (*API).replaceTodo},
	{Route{http.MethodPatch, "/api/todos/{id}"}, (*API).patchTodo},
	{Route{http.MethodDelete, "/api/todos/{id}"}, (*API).deleteTodo},
}

// Routes lists the API endpoints, excluding the documentation pages.
func Routes() []Route {
	routes := make([]Route, len(endpoints))
	for i, e := range endpoints {
		routes[i] = e.Route
	}
	return routes
}

// NewRouter returns the HTTP handler with every route registered, backed by
// the given store.
func NewRouter(store todo.Store) http.Handler {
	api := &API{store: store}

	mux := http.NewServeMux()
	docs.Register(mux, todoapi.Spec)
	for _, e := range endpoints {
		mux.HandleFunc(e.Method+" "+e.Path, func(w http.ResponseWriter, r *http.Request) {
			e.handle(api, w, r)
		})
	}
	return mux
}
