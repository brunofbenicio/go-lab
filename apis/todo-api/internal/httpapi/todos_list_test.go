package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestListTodosEmpty(t *testing.T) {
	h := newTestRouter(t)

	rec := do(t, h, http.MethodGet, "/api/todos", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// An empty collection must be [] and not null, which breaks many clients.
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("body = %s, want an empty items array", rec.Body.String())
	}
	var got listResponse
	decode(t, rec, &got)
	if got.Total != 0 || len(got.Items) != 0 {
		t.Errorf("got %+v, want no items", got)
	}
}

func TestListTodos(t *testing.T) {
	h := newTestRouter(t)
	for _, title := range []string{"one", "two", "three"} {
		do(t, h, http.MethodPost, "/api/todos", `{"title":"`+title+`"}`)
	}

	rec := do(t, h, http.MethodGet, "/api/todos", "")

	var got listResponse
	decode(t, rec, &got)
	if got.Total != 3 || len(got.Items) != 3 {
		t.Fatalf("got total %d with %d items, want 3 and 3", got.Total, len(got.Items))
	}
	for i, want := range []string{"one", "two", "three"} {
		if got.Items[i].Title != want || got.Items[i].ID != int64(i+1) {
			t.Errorf("item %d = %+v, want id %d title %q", i, got.Items[i], i+1, want)
		}
	}
}
