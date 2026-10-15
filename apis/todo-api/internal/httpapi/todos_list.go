package httpapi

import (
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// listResponse is the body of GET /api/todos.
type listResponse struct {
	Items []todo.Todo `json:"items"`
	Total int         `json:"total"`
}

// listTodos handles GET /api/todos.
func (a *API) listTodos(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.List(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if items == nil {
		items = []todo.Todo{} // encode as [] rather than null
	}

	writeJSON(w, http.StatusOK, listResponse{Items: items, Total: len(items)})
}
