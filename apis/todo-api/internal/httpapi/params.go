package httpapi

import (
	"net/http"
	"strconv"
)

// todoID extracts the {id} path value. When it is not a positive integer it
// writes a 400 response and reports false.
func todoID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_id", "the todo id must be a positive integer")
		return 0, false
	}
	return id, true
}

// writeNotFound answers 404 for a missing todo.
func writeNotFound(w http.ResponseWriter, id int64) {
	writeError(w, http.StatusNotFound, "not_found", "todo "+strconv.FormatInt(id, 10)+" was not found")
}
