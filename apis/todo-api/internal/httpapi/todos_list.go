package httpapi

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// listResponse is the body of GET /api/todos.
type listResponse struct {
	Items  []todo.Todo `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// listTodos handles GET /api/todos.
//
// Query parameters: done=true|false, q=<text>, limit=1..200 (default 50) and
// offset>=0. The X-Total-Count header repeats the total number of matches.
func (a *API) listTodos(w http.ResponseWriter, r *http.Request) {
	opts, problems := parseListOptions(r.URL.Query())
	if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, "invalid_query", "one or more query parameters are invalid", problems...)
		return
	}

	result, err := a.store.List(r.Context(), opts)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if result.Items == nil {
		result.Items = []todo.Todo{} // encode as [] rather than null
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(result.Total))
	writeJSON(w, http.StatusOK, listResponse{
		Items:  result.Items,
		Total:  result.Total,
		Limit:  opts.Limit,
		Offset: opts.Offset,
	})
}

// parseListOptions reads and validates the list query parameters.
func parseListOptions(q url.Values) (todo.ListOptions, []fieldError) {
	opts := todo.ListOptions{Limit: defaultLimit, Query: q.Get("q")}
	var problems []fieldError

	if raw := q.Get("done"); raw != "" {
		done, err := strconv.ParseBool(raw)
		if err != nil {
			problems = append(problems, fieldError{Field: "done", Message: "must be true or false"})
		} else {
			opts.Done = &done
		}
	}

	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxLimit {
			problems = append(problems, fieldError{Field: "limit", Message: "must be an integer between 1 and " + strconv.Itoa(maxLimit)})
		} else {
			opts.Limit = limit
		}
	}

	if raw := q.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			problems = append(problems, fieldError{Field: "offset", Message: "must be an integer greater than or equal to 0"})
		} else {
			opts.Offset = offset
		}
	}

	return opts, problems
}
