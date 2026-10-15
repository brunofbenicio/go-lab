package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func seedTodos(t *testing.T, h http.Handler) {
	t.Helper()
	bodies := []string{
		`{"title":"Buy milk"}`,
		`{"title":"Write report","done":true}`,
		`{"title":"Call the bank","notes":"milk subscription","done":true}`,
		`{"title":"Water plants"}`,
		`{"title":"Plan trip","done":true}`,
	}
	for _, b := range bodies {
		if rec := do(t, h, http.MethodPost, "/api/todos", b); rec.Code != http.StatusCreated {
			t.Fatalf("seeding failed: %d %s", rec.Code, rec.Body.String())
		}
	}
}

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
	if got := rec.Header().Get("X-Total-Count"); got != "0" {
		t.Errorf("X-Total-Count = %q, want 0", got)
	}
}

func TestListTodos(t *testing.T) {
	h := newTestRouter(t)
	seedTodos(t, h)

	tests := []struct {
		name      string
		query     string
		wantIDs   []int64
		wantTotal int
		wantLimit int
	}{
		{name: "defaults", query: "", wantIDs: []int64{1, 2, 3, 4, 5}, wantTotal: 5, wantLimit: 50},
		{name: "done only", query: "done=true", wantIDs: []int64{2, 3, 5}, wantTotal: 3, wantLimit: 50},
		{name: "open only", query: "done=false", wantIDs: []int64{1, 4}, wantTotal: 2, wantLimit: 50},
		{name: "search", query: "q=MILK", wantIDs: []int64{1, 3}, wantTotal: 2, wantLimit: 50},
		{name: "second page", query: "limit=2&offset=2", wantIDs: []int64{3, 4}, wantTotal: 5, wantLimit: 2},
		{name: "filters and paging together", query: "done=true&limit=1&offset=1", wantIDs: []int64{3}, wantTotal: 3, wantLimit: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := "/api/todos"
			if tt.query != "" {
				target += "?" + tt.query
			}

			rec := do(t, h, http.MethodGet, target, "")

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			var got listResponse
			decode(t, rec, &got)
			if len(got.Items) != len(tt.wantIDs) {
				t.Fatalf("got %d items, want %d", len(got.Items), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got.Items[i].ID != id {
					t.Errorf("item %d has id %d, want %d", i, got.Items[i].ID, id)
				}
			}
			if got.Total != tt.wantTotal || got.Limit != tt.wantLimit {
				t.Errorf("total = %d, limit = %d, want %d and %d", got.Total, got.Limit, tt.wantTotal, tt.wantLimit)
			}
			if hdr := rec.Header().Get("X-Total-Count"); hdr != strconv.Itoa(tt.wantTotal) {
				t.Errorf("X-Total-Count = %q, want %d", hdr, tt.wantTotal)
			}
		})
	}
}

func TestListTodosInvalidQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantField string
	}{
		{name: "done not a bool", query: "done=maybe", wantField: "done"},
		{name: "limit not a number", query: "limit=ten", wantField: "limit"},
		{name: "limit zero", query: "limit=0", wantField: "limit"},
		{name: "limit above the maximum", query: "limit=201", wantField: "limit"},
		{name: "negative offset", query: "offset=-1", wantField: "offset"},
	}

	h := newTestRouter(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, "/api/todos?"+tt.query, "")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			var body errorBody
			decode(t, rec, &body)
			if body.Error.Code != "invalid_query" || len(body.Error.Details) != 1 || body.Error.Details[0].Field != tt.wantField {
				t.Errorf("error = %+v, want invalid_query pointing at %q", body.Error, tt.wantField)
			}
		})
	}
}
