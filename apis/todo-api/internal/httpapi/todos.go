package httpapi

import (
	"fmt"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// API groups the HTTP handlers that operate on todos.
type API struct {
	store todo.Store
}

// createTodo handles POST /api/todos.
func (a *API) createTodo(w http.ResponseWriter, r *http.Request) {
	var in todo.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}

	in = in.Normalized()
	if err := in.Validate(); err != nil {
		writeValidationError(w, err)
		return
	}

	created, err := a.store.Create(r.Context(), in)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/todos/%d", created.ID))
	writeJSON(w, http.StatusCreated, created)
}
