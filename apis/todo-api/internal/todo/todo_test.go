package todo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInputValidate(t *testing.T) {
	zero := time.Time{}
	due := time.Date(2026, 12, 24, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		input      Input
		wantFields []string
	}{
		{name: "minimal valid", input: Input{Title: "Buy milk"}},
		{name: "fully populated", input: Input{Title: "Ship it", Notes: "before friday", Done: true, DueDate: &due}},
		{name: "title with accents counts characters", input: Input{Title: strings.Repeat("ç", MaxTitleLength)}},
		{name: "empty title", input: Input{}, wantFields: []string{"title"}},
		{name: "blank title", input: Input{Title: "   \t"}, wantFields: []string{"title"}},
		{name: "title too long", input: Input{Title: strings.Repeat("a", MaxTitleLength+1)}, wantFields: []string{"title"}},
		{name: "notes too long", input: Input{Title: "ok", Notes: strings.Repeat("n", MaxNotesLength+1)}, wantFields: []string{"notes"}},
		{name: "zero due date", input: Input{Title: "ok", DueDate: &zero}, wantFields: []string{"due_date"}},
		{name: "reports every problem", input: Input{Notes: strings.Repeat("n", MaxNotesLength+1)}, wantFields: []string{"title", "notes"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if len(tt.wantFields) == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}

			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() = %v, want *ValidationError", err)
			}
			if len(verr.Fields) != len(tt.wantFields) {
				t.Fatalf("got %d field errors (%v), want %d", len(verr.Fields), verr.Fields, len(tt.wantFields))
			}
			for i, want := range tt.wantFields {
				if verr.Fields[i].Field != want {
					t.Errorf("field error %d is for %q, want %q", i, verr.Fields[i].Field, want)
				}
			}
		})
	}
}

func TestValidationErrorMessage(t *testing.T) {
	err := &ValidationError{Fields: []FieldError{{"title", "must not be empty"}, {"notes", "too long"}}}

	want := "invalid todo: title: must not be empty; notes: too long"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
