package httpapi

import (
	"errors"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// getTodo handles GET /api/todos/{id}.
func (a *API) getTodo(w http.ResponseWriter, r *http.Request) {
	id, ok := todoID(w, r)
	if !ok {
		return
	}

	found, err := a.store.Get(r.Context(), id)
	switch {
	case errors.Is(err, todo.ErrNotFound):
		writeNotFound(w, id)
	case err != nil:
		writeInternalError(w, err)
	default:
		writeJSON(w, http.StatusOK, found)
	}
}
