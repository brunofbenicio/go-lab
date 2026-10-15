package todoapi_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	todoapi "github.com/brunofbenicio/go-lab/apis/todo-api"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/httpapi"
)

var (
	pathLine   = regexp.MustCompile(`^  (/\S*):\s*$`)
	methodLine = regexp.MustCompile(`^    (get|put|post|delete|patch):\s*$`)
)

// documentedOperations extracts "METHOD /path" pairs from the paths section of
// the spec. It relies on the document's layout (paths at two spaces, methods at
// four) because the standard library cannot parse YAML.
func documentedOperations(t *testing.T) []string {
	t.Helper()

	var ops []string
	var currentPath string
	inPaths := false

	for _, line := range strings.Split(string(todoapi.Spec), "\n") {
		switch {
		case line == "paths:":
			inPaths = true
		case inPaths && line != "" && !strings.HasPrefix(line, " "):
			inPaths = false // the next top-level key ends the section
		case !inPaths:
		default:
			if m := pathLine.FindStringSubmatch(line); m != nil {
				currentPath = m[1]
			} else if m := methodLine.FindStringSubmatch(line); m != nil && currentPath != "" {
				ops = append(ops, strings.ToUpper(m[1])+" "+currentPath)
			}
		}
	}
	sort.Strings(ops)
	return ops
}

func routedOperations() []string {
	var ops []string
	for _, r := range httpapi.Routes() {
		ops = append(ops, r.Method+" "+r.Path)
	}
	sort.Strings(ops)
	return ops
}

func TestEveryRouteIsDocumented(t *testing.T) {
	documented := make(map[string]bool)
	for _, op := range documentedOperations(t) {
		documented[op] = true
	}
	if len(documented) == 0 {
		t.Fatal("no operations found in the spec, did the layout of openapi.yaml change?")
	}

	for _, op := range routedOperations() {
		if !documented[op] {
			t.Errorf("route %q is served but missing from openapi.yaml", op)
		}
	}
}

func TestEveryDocumentedOperationIsRouted(t *testing.T) {
	routed := make(map[string]bool)
	for _, op := range routedOperations() {
		routed[op] = true
	}

	for _, op := range documentedOperations(t) {
		if !routed[op] {
			t.Errorf("operation %q is documented in openapi.yaml but the server does not route it", op)
		}
	}
}

func TestDocumentationHelperFindsTheExpectedShape(t *testing.T) {
	ops := documentedOperations(t)

	if len(ops) != len(httpapi.Routes()) {
		t.Errorf("found %d documented operations (%v), want %d to match the routes", len(ops), ops, len(httpapi.Routes()))
	}
}
