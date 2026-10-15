package jwtauth

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/docs"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/httpapi"
)

var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// documented lists the operations declared under "paths:" as path -> methods.
// It is a light, string-based reading of the document: paths are keys with two
// spaces of indentation and operations are HTTP method keys with four.
func documented(spec string) map[string][]string {
	ops := make(map[string][]string)
	inPaths := false
	current := ""
	for _, line := range strings.Split(spec, "\n") {
		if line == "paths:" {
			inPaths = true
			continue
		}
		if inPaths && line != "" && !strings.HasPrefix(line, " ") {
			inPaths = false
		}
		if !inPaths {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		key, isKey := strings.CutSuffix(strings.TrimSpace(line), ":")
		switch {
		case !isKey:
		case indent == 2 && strings.HasPrefix(key, "/"):
			current = key
			ops[current] = nil
		case indent == 4 && current != "" && slices.Contains(httpMethods, key):
			ops[current] = append(ops[current], key)
		}
	}
	return ops
}

func TestSpecIsOpenAPI303(t *testing.T) {
	if !strings.HasPrefix(string(OpenAPI()), "openapi: 3.0.3\n") {
		t.Error("openapi.yaml must start with \"openapi: 3.0.3\"")
	}
}

func TestOpenAPIReturnsACopy(t *testing.T) {
	first := OpenAPI()
	first[0] = 'X'
	if OpenAPI()[0] == 'X' {
		t.Error("mutating the returned slice changed the embedded document")
	}
}

func TestEveryRegisteredRouteIsDocumented(t *testing.T) {
	ops := documented(string(OpenAPI()))
	routes := slices.Concat(httpapi.Routes(), docs.Routes())

	for _, pattern := range routes {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("malformed route pattern %q", pattern)
		}
		methods, found := ops[path]
		if !found {
			t.Errorf("route %q: path %s is missing from openapi.yaml", pattern, path)
			continue
		}
		if !slices.Contains(methods, strings.ToLower(method)) {
			t.Errorf("route %q: openapi.yaml documents %s with %v only", pattern, path, methods)
		}
	}
}

func TestEveryDocumentedOperationIsRegistered(t *testing.T) {
	registered := slices.Concat(httpapi.Routes(), docs.Routes())

	for path, methods := range documented(string(OpenAPI())) {
		for _, method := range methods {
			want := strings.ToUpper(method) + " " + path
			if !slices.Contains(registered, want) {
				t.Errorf("openapi.yaml documents %q, which no handler registers", want)
			}
		}
	}
}

func TestOperationIDsAreUnique(t *testing.T) {
	spec := string(OpenAPI())
	ids := regexp.MustCompile(`(?m)^ {6}operationId: (\S+)$`).FindAllStringSubmatch(spec, -1)

	operations := 0
	for _, methods := range documented(spec) {
		operations += len(methods)
	}
	if len(ids) != operations {
		t.Errorf("found %d operationId entries for %d operations", len(ids), operations)
	}

	seen := make(map[string]bool)
	for _, m := range ids {
		if seen[m[1]] {
			t.Errorf("operationId %q is used twice", m[1])
		}
		seen[m[1]] = true
	}
}

func TestReferencesResolve(t *testing.T) {
	spec := string(OpenAPI())
	refs := regexp.MustCompile(`\$ref: '#/components/(\w+)/(\w+)'`).FindAllStringSubmatch(spec, -1)
	if len(refs) == 0 {
		t.Fatal("no $ref found; the document should reuse its components")
	}

	for _, m := range refs {
		kind, name := m[1], m[2]
		kindStart := strings.Index(spec, "\n  "+kind+":\n")
		if kindStart < 0 {
			t.Errorf("$ref to components/%s/%s: section %q does not exist", kind, name, kind)
			continue
		}
		if !strings.Contains(spec[kindStart:], "\n    "+name+":\n") {
			t.Errorf("$ref to components/%s/%s does not resolve", kind, name)
		}
	}
}
