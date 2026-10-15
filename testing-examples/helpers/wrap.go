// Package wrap breaks text into lines of a maximum width, the way a terminal
// help screen or an e-mail formatter would.
package wrap

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrWidth is returned when the requested width is not positive.
var ErrWidth = errors.New("wrap: width must be positive")

// Wrap splits text into lines of at most width runes, breaking only at
// whitespace. A single word longer than width is kept whole on its own line.
// Runs of whitespace collapse into a single space.
func Wrap(text string, width int) ([]string, error) {
	if width <= 0 {
		return nil, ErrWidth
	}
	var (
		lines []string
		line  strings.Builder
		size  int // runes in line
	)
	flush := func() {
		if size > 0 {
			lines = append(lines, line.String())
			line.Reset()
			size = 0
		}
	}
	for _, word := range strings.Fields(text) {
		n := utf8.RuneCountInString(word)
		if size > 0 && size+1+n > width {
			flush()
		}
		if size > 0 {
			line.WriteByte(' ')
			size++
		}
		line.WriteString(word)
		size += n
	}
	flush()
	return lines, nil
}
