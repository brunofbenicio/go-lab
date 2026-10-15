// Package todoapi is a minimal JSON API for a to-do list, written as plain
// net/http handlers so they can be tested without starting a server.
package todoapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Todo is a task.
type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Server stores to-dos in memory. Create it with NewServer.
type Server struct {
	mu     sync.Mutex
	todos  map[int]Todo
	nextID int
}

// NewServer returns an empty Server.
func NewServer() *Server {
	return &Server{todos: make(map[int]Todo), nextID: 1}
}

// Handler returns the routes of the API:
//
//	GET    /todos        list all (sorted by id)
//	POST   /todos        create from {"title": "..."}
//	GET    /todos/{id}   fetch one
//	DELETE /todos/{id}   remove one
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /todos", s.list)
	mux.HandleFunc("POST /todos", s.create)
	mux.HandleFunc("GET /todos/{id}", s.get)
	mux.HandleFunc("DELETE /todos/{id}", s.remove)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	out := make([]Todo, 0, len(s.todos))
	for _, t := range s.todos {
		out = append(out, t)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		writeError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}
	s.mu.Lock()
	t := Todo{ID: s.nextID, Title: in.Title}
	s.todos[t.ID] = t
	s.nextID++
	s.mu.Unlock()

	w.Header().Set("Location", "/todos/"+strconv.Itoa(t.ID))
	writeJSON(w, http.StatusCreated, t)
}

// lookup parses the {id} path value and finds the to-do, writing the error
// response itself when it cannot.
func (s *Server) lookup(w http.ResponseWriter, r *http.Request) (Todo, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be a number")
		return Todo{}, false
	}
	s.mu.Lock()
	t, ok := s.todos[id]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "todo not found")
		return Todo{}, false
	}
	return t, true
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	if t, ok := s.lookup(w, r); ok {
		writeJSON(w, http.StatusOK, t)
	}
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	t, ok := s.lookup(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	delete(s.todos, t.ID)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
