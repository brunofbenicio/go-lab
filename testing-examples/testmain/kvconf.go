// Package kvconf reads simple "key = value" configuration files.
//
// Blank lines and lines starting with '#' are ignored. Whitespace around keys
// and values is trimmed. A later assignment to the same key wins.
package kvconf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrSyntax is returned (wrapped) for lines that are not valid assignments.
var ErrSyntax = errors.New("kvconf: syntax error")

// Parse reads configuration from r.
func Parse(r io.Reader) (map[string]string, error) {
	conf := make(map[string]string)
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("%w: line %d: %q", ErrSyntax, line, text)
		}
		conf[key] = strings.TrimSpace(value)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return conf, nil
}

// Load parses the file at path.
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	conf, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return conf, nil
}
