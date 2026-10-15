package todo

import (
	"context"
	"reflect"
	"testing"
)

func seededStore(t *testing.T) *MemoryStore {
	t.Helper()
	s := NewMemoryStore()
	inputs := []Input{
		{Title: "Buy milk", Notes: "semi-skimmed"},
		{Title: "Write report", Done: true},
		{Title: "Call the bank", Notes: "about the MILK subscription", Done: true},
		{Title: "Water plants"},
		{Title: "Plan trip", Done: true},
	}
	for _, in := range inputs {
		if _, err := s.Create(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func ids(items []Todo) []int64 {
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestMemoryStoreListFilterAndPaging(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name      string
		opts      ListOptions
		wantIDs   []int64
		wantTotal int
	}{
		{name: "no options", opts: ListOptions{}, wantIDs: []int64{1, 2, 3, 4, 5}, wantTotal: 5},
		{name: "only done", opts: ListOptions{Done: &yes}, wantIDs: []int64{2, 3, 5}, wantTotal: 3},
		{name: "only open", opts: ListOptions{Done: &no}, wantIDs: []int64{1, 4}, wantTotal: 2},
		{name: "query matches title and notes ignoring case", opts: ListOptions{Query: "milk"}, wantIDs: []int64{1, 3}, wantTotal: 2},
		{name: "query and done combine", opts: ListOptions{Query: "milk", Done: &yes}, wantIDs: []int64{3}, wantTotal: 1},
		{name: "query without matches", opts: ListOptions{Query: "zebra"}, wantIDs: []int64{}, wantTotal: 0},
		{name: "limit", opts: ListOptions{Limit: 2}, wantIDs: []int64{1, 2}, wantTotal: 5},
		{name: "offset and limit", opts: ListOptions{Offset: 2, Limit: 2}, wantIDs: []int64{3, 4}, wantTotal: 5},
		{name: "last partial page", opts: ListOptions{Offset: 4, Limit: 10}, wantIDs: []int64{5}, wantTotal: 5},
		{name: "offset beyond the end", opts: ListOptions{Offset: 50}, wantIDs: []int64{}, wantTotal: 5},
		{name: "negative offset is treated as zero", opts: ListOptions{Offset: -3, Limit: 1}, wantIDs: []int64{1}, wantTotal: 5},
		{name: "paging applies after filtering", opts: ListOptions{Done: &yes, Offset: 1, Limit: 1}, wantIDs: []int64{3}, wantTotal: 3},
	}

	s := seededStore(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.List(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if !reflect.DeepEqual(ids(got.Items), tt.wantIDs) {
				t.Errorf("IDs = %v, want %v", ids(got.Items), tt.wantIDs)
			}
			if got.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", got.Total, tt.wantTotal)
			}
		})
	}
}
