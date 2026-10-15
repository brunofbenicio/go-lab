package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// discardWriter is a ResponseWriter that throws the body away, so the
// benchmarks measure routing, decoding and encoding rather than buffer growth.
type discardWriter struct {
	header http.Header
	status int
}

func newDiscardWriter() *discardWriter { return &discardWriter{header: make(http.Header)} }

func (w *discardWriter) Header() http.Header         { return w.header }
func (w *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *discardWriter) WriteHeader(status int)      { w.status = status }

func benchRouter(b *testing.B, n int) http.Handler {
	b.Helper()
	store := todo.NewMemoryStore()
	for i := 0; i < n; i++ {
		if _, err := store.Create(b.Context(), todo.Input{Title: fmt.Sprintf("task %d", i), Notes: "notes"}); err != nil {
			b.Fatal(err)
		}
	}
	return NewRouter(store)
}

func BenchmarkGetTodo(b *testing.B) {
	h := benchRouter(b, 100)
	req := httptest.NewRequest(http.MethodGet, "/api/todos/42", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(newDiscardWriter(), req)
	}
}

func BenchmarkCreateTodo(b *testing.B) {
	h := benchRouter(b, 0)
	const body = `{"title":"benchmark todo","notes":"created in a loop","done":false}`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/todos", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := newDiscardWriter()
		h.ServeHTTP(w, req)
		if w.status != http.StatusCreated {
			b.Fatalf("status = %d, want 201", w.status)
		}
	}
}

func BenchmarkListTodos(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			h := benchRouter(b, n)
			req := httptest.NewRequest(http.MethodGet, "/api/todos?limit=50", nil)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.ServeHTTP(newDiscardWriter(), req)
			}
		})
	}
}

func BenchmarkGetTodoParallel(b *testing.B) {
	h := benchRouter(b, 100)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/api/todos/7", nil)
		for pb.Next() {
			h.ServeHTTP(newDiscardWriter(), req)
		}
	})
}
