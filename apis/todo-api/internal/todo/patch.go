package todo

import (
	"bytes"
	"encoding/json"
	"time"
)

// OptionalTime distinguishes a JSON field that was omitted from one that was
// explicitly set to null, which a plain *time.Time cannot do.
type OptionalTime struct {
	// Set is true when the field appeared in the JSON document.
	Set bool
	// Value is nil when the field was null, i.e. the due date must be cleared.
	Value *time.Time
}

// UnmarshalJSON implements json.Unmarshaler. It is not called for omitted
// fields, which is exactly how Set stays false for them.
func (o *OptionalTime) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}

	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	o.Value = &t
	return nil
}

// Patch is a partial update: nil pointers and an unset DueDate mean
// "leave the field as it is".
type Patch struct {
	Title   *string      `json:"title"`
	Notes   *string      `json:"notes"`
	Done    *bool        `json:"done"`
	DueDate OptionalTime `json:"due_date"`
}

// IsEmpty reports whether the patch changes nothing.
func (p Patch) IsEmpty() bool {
	return p.Title == nil && p.Notes == nil && p.Done == nil && !p.DueDate.Set
}

// Apply returns the Input that results from applying the patch on top of t.
func (p Patch) Apply(t Todo) Input {
	in := Input{Title: t.Title, Notes: t.Notes, Done: t.Done, DueDate: copyTime(t.DueDate)}
	if p.Title != nil {
		in.Title = *p.Title
	}
	if p.Notes != nil {
		in.Notes = *p.Notes
	}
	if p.Done != nil {
		in.Done = *p.Done
	}
	if p.DueDate.Set {
		in.DueDate = copyTime(p.DueDate.Value)
	}
	return in
}
