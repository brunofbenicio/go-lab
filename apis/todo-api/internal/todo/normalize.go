package todo

import "strings"

// Normalized returns a copy of the input with surrounding whitespace removed
// from the free-text fields, ready to be validated and stored.
func (in Input) Normalized() Input {
	in.Title = strings.TrimSpace(in.Title)
	in.Notes = strings.TrimSpace(in.Notes)
	return in
}
