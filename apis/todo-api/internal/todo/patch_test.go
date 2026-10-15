package todo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPatchJSONDistinguishesMissingFromNull(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		wantEmpty    bool
		wantDueSet   bool
		wantDueClear bool
	}{
		{name: "empty object", json: `{}`, wantEmpty: true},
		{name: "due date omitted", json: `{"done":true}`},
		{name: "due date null clears", json: `{"due_date":null}`, wantDueSet: true, wantDueClear: true},
		{name: "due date value", json: `{"due_date":"2026-03-04T05:06:07Z"}`, wantDueSet: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p Patch
			if err := json.Unmarshal([]byte(tt.json), &p); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if p.IsEmpty() != tt.wantEmpty {
				t.Errorf("IsEmpty() = %v, want %v", p.IsEmpty(), tt.wantEmpty)
			}
			if p.DueDate.Set != tt.wantDueSet {
				t.Errorf("DueDate.Set = %v, want %v", p.DueDate.Set, tt.wantDueSet)
			}
			if tt.wantDueSet && (p.DueDate.Value == nil) != tt.wantDueClear {
				t.Errorf("DueDate.Value nil = %v, want %v", p.DueDate.Value == nil, tt.wantDueClear)
			}
		})
	}
}

func TestPatchRejectsInvalidDueDate(t *testing.T) {
	var p Patch
	if err := json.Unmarshal([]byte(`{"due_date":"soon"}`), &p); err == nil {
		t.Error("Unmarshal() error = nil, want an error for a malformed timestamp")
	}
}

func TestPatchApply(t *testing.T) {
	due := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	base := Todo{ID: 1, Title: "base", Notes: "notes", Done: false, DueDate: &due}
	newTitle, done := "changed", true

	got := Patch{Title: &newTitle, Done: &done}.Apply(base)
	if got.Title != "changed" || !got.Done || got.Notes != "notes" || got.DueDate == nil {
		t.Errorf("Apply() = %+v, want title and done changed, notes and due date kept", got)
	}

	cleared := Patch{DueDate: OptionalTime{Set: true}}.Apply(base)
	if cleared.DueDate != nil {
		t.Errorf("DueDate = %v, want it cleared", cleared.DueDate)
	}
	if base.DueDate == nil {
		t.Error("Apply() must not modify the original Todo")
	}
}

func TestMemoryStorePatch(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStoreWithClock(fakeClock())
	created, _ := s.Create(ctx, Input{Title: "original", Notes: "keep me"})
	done := true
	blank := "   "

	patched, err := s.Patch(ctx, created.ID, Patch{Done: &done})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if !patched.Done || patched.Title != "original" || patched.Notes != "keep me" {
		t.Errorf("Patch() = %+v, want only done changed", patched)
	}
	if !patched.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt %v should move past %v", patched.UpdatedAt, created.UpdatedAt)
	}

	noop, err := s.Patch(ctx, created.ID, Patch{})
	if err != nil {
		t.Fatalf("empty Patch() error = %v", err)
	}
	if !noop.UpdatedAt.Equal(patched.UpdatedAt) {
		t.Errorf("an empty patch must not touch UpdatedAt, got %v want %v", noop.UpdatedAt, patched.UpdatedAt)
	}

	var verr *ValidationError
	if _, err := s.Patch(ctx, created.ID, Patch{Title: &blank}); !errors.As(err, &verr) {
		t.Errorf("Patch() with blank title error = %v, want *ValidationError", err)
	}
	stored, _ := s.Get(ctx, created.ID)
	if stored.Title != "original" {
		t.Errorf("a rejected patch must not be applied, title = %q", stored.Title)
	}

	if _, err := s.Patch(ctx, 99, Patch{Done: &done}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Patch() on missing todo error = %v, want ErrNotFound", err)
	}
}
