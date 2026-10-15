package httpapi

import (
	"errors"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// patchTodo handles PATCH /api/todos/{id}. Only the fields present in the body
// change; "due_date": null clears the due date.
func (a *API) patchTodo(w http.ResponseWriter, r *http.Request) {
	id, ok := todoID(w, r)
	if !ok {
		return
	}

	var p todo.Patch
	if err := decodeJSON(w, r, &p); err != nil {
		writeDecodeError(w, err)
		return
	}

	updated, err := a.store.Patch(r.Context(), id, p)
	var verr *todo.ValidationError
	switch {
	case errors.Is(err, todo.ErrNotFound):
		writeNotFound(w, id)
	case errors.As(err, &verr):
		writeValidationError(w, err)
	case err != nil:
		writeInternalError(w, err)
	default:
		writeJSON(w, http.StatusOK, updated)
	}
}
