// Package todo holds the domain model of the todo API: the Todo entity, the
// input accepted from clients and its validation rules.
package todo

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxTitleLength is the longest title accepted, in characters.
	MaxTitleLength = 120
	// MaxNotesLength is the longest notes text accepted, in characters.
	MaxNotesLength = 1000
)

// Todo is a task stored by the API.
type Todo struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Notes     string     `json:"notes,omitempty"`
	Done      bool       `json:"done"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Input carries the fields a client may set when creating or replacing a Todo.
type Input struct {
	Title   string     `json:"title"`
	Notes   string     `json:"notes"`
	Done    bool       `json:"done"`
	DueDate *time.Time `json:"due_date"`
}

// FieldError describes one invalid field.
type FieldError struct {
	Field   string
	Message string
}

// ValidationError lists every problem found in an Input.
type ValidationError struct {
	Fields []FieldError
}

// Error implements error.
func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "invalid todo: " + strings.Join(parts, "; ")
}

// Validate checks the input against the business rules. It returns a
// *ValidationError describing every violation, or nil when the input is valid.
func (in Input) Validate() error {
	var problems []FieldError

	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		problems = append(problems, FieldError{"title", "must not be empty"})
	case utf8.RuneCountInString(title) > MaxTitleLength:
		problems = append(problems, FieldError{"title", fmt.Sprintf("must be at most %d characters", MaxTitleLength)})
	}

	if utf8.RuneCountInString(in.Notes) > MaxNotesLength {
		problems = append(problems, FieldError{"notes", fmt.Sprintf("must be at most %d characters", MaxNotesLength)})
	}

	if in.DueDate != nil && in.DueDate.IsZero() {
		problems = append(problems, FieldError{"due_date", "must be a valid RFC 3339 timestamp"})
	}

	if len(problems) > 0 {
		return &ValidationError{Fields: problems}
	}
	return nil
}
