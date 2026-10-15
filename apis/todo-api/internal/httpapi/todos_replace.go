package httpapi

import (
	"errors"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// replaceTodo handles PUT /api/todos/{id}. The body replaces every
// client-controlled field, so omitted fields are reset to their zero value.
func (a *API) replaceTodo(w http.ResponseWriter, r *http.Request) {
	id, ok := todoID(w, r)
	if !ok {
		return
	}

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

	updated, err := a.store.Replace(r.Context(), id, in)
	switch {
	case errors.Is(err, todo.ErrNotFound):
		writeNotFound(w, id)
	case err != nil:
		writeInternalError(w, err)
	default:
		writeJSON(w, http.StatusOK, updated)
	}
}
