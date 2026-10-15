package httpapi

import (
	"errors"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// deleteTodo handles DELETE /api/todos/{id} and answers 204 with no body.
func (a *API) deleteTodo(w http.ResponseWriter, r *http.Request) {
	id, ok := todoID(w, r)
	if !ok {
		return
	}

	err := a.store.Delete(r.Context(), id)
	switch {
	case errors.Is(err, todo.ErrNotFound):
		writeNotFound(w, id)
	case err != nil:
		writeInternalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
