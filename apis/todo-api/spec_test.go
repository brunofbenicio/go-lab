package todoapi

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestSpecIsEmbedded(t *testing.T) {
	if len(Spec) == 0 {
		t.Fatal("Spec is empty, the OpenAPI document was not embedded")
	}
	if !bytes.HasPrefix(Spec, []byte("openapi: 3.0.3")) {
		t.Errorf("Spec should start with the OpenAPI version, starts with %.30q", Spec)
	}
	if bytes.Contains(Spec, []byte("\t")) {
		t.Error("YAML must not contain tab characters")
	}
}

func TestSpecDescribesEveryOperation(t *testing.T) {
	operations := []string{
		"getHealth", "listTodos", "createTodo", "getTodo", "replaceTodo", "patchTodo", "deleteTodo",
	}
	for _, id := range operations {
		if !bytes.Contains(Spec, []byte("operationId: "+id+"\n")) {
			t.Errorf("operation %q is missing from the spec", id)
		}
	}
}

// TestSpecReferencesResolve checks that every `$ref: '#/components/<kind>/<name>'`
// points at an entry that exists. The standard library has no YAML parser, so
// this looks for the entry as a four-space-indented key, which is how the
// document lays out its components.
func TestSpecReferencesResolve(t *testing.T) {
	text := string(Spec)
	components := text[strings.Index(text, "\ncomponents:"):]

	refs := regexp.MustCompile(`\$ref: '#/components/([A-Za-z]+)/([A-Za-z0-9_]+)'`).FindAllStringSubmatch(text, -1)
	if len(refs) == 0 {
		t.Fatal("the spec should use $ref to share components")
	}

	for _, ref := range refs {
		kind, name := ref[1], ref[2]
		section := components[strings.Index(components, "\n  "+kind+":"):]
		if !strings.Contains(section, "\n    "+name+":") {
			t.Errorf("$ref to components/%s/%s does not resolve", kind, name)
		}
	}
}
