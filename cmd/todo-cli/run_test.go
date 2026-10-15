package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runCLI runs todo-cli against the given task file.
func runCLI(path string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	full := append([]string{"-f", path}, args...)
	code = run(full, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

// TestWorkflow drives one task file through a realistic sequence of commands.
func TestWorkflow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.json")
	steps := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    string
		wantErrHas string
	}{
		{"list on a new file", []string{"list"}, 0, "no tasks\n", ""},
		{"add first", []string{"add", "buy milk"}, 0, "added #1: buy milk\n", ""},
		{"add second", []string{"add", "write tests"}, 0, "added #2: write tests\n", ""},
		{"title may span several arguments", []string{"add", "call", "mom"}, 0, "added #3: call mom\n", ""},
		{"list all", []string{"list"}, 0, "[ ] 1 buy milk\n[ ] 2 write tests\n[ ] 3 call mom\n", ""},
		{"complete a task", []string{"done", "2"}, 0, "completed #2: write tests\n", ""},
		{"list pending", []string{"list", "-status", "pending"}, 0, "[ ] 1 buy milk\n[ ] 3 call mom\n", ""},
		{"list done", []string{"list", "-status", "done"}, 0, "[x] 2 write tests\n", ""},
		{"ls alias", []string{"ls", "-status", "done"}, 0, "[x] 2 write tests\n", ""},
		{"remove a task", []string{"rm", "1"}, 0, "removed #1: buy milk\n", ""},
		{"ids are not reused below the maximum", []string{"add", "new"}, 0, "added #4: new\n", ""},
		{"final list", []string{"list"}, 0, "[x] 2 write tests\n[ ] 3 call mom\n[ ] 4 new\n", ""},
		{"complete unknown task", []string{"done", "99"}, 1, "", "task 99 not found"},
		{"remove unknown task", []string{"rm", "99"}, 1, "", "task 99 not found"},
		{"the last -status flag wins", []string{"list", "-status", "done", "-status", "pending"}, 0, "[ ] 3 call mom\n[ ] 4 new\n", ""},
		{"a dash is allowed at the start of a title", []string{"add", "-review", "notes"}, 0, "added #5: -review notes\n", ""},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(path, s.args...)
			if code != s.wantCode || stdout != s.wantOut {
				t.Errorf("run(%q) = (%d, %q), want (%d, %q); stderr %q", s.args, code, stdout, s.wantCode, s.wantOut, stderr)
			}
			if !strings.Contains(stderr, s.wantErrHas) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, s.wantErrHas)
			}
		})
	}

	// The file itself must hold the final state as plain JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []Task
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("task file is not valid JSON: %v\n%s", err, data)
	}
	want := []Task{{2, "write tests", true}, {3, "call mom", false}, {4, "new", false}, {5, "-review notes", false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stored tasks = %+v, want %+v", got, want)
	}
}

func TestUsageErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.json")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantErrHas string
	}{
		{"no command", nil, 2, "usage: todo-cli"},
		{"unknown command", []string{"explode"}, 2, "unknown command"},
		{"add without title", []string{"add"}, 2, "add needs a title"},
		{"add with blank title", []string{"add", "  "}, 2, "add needs a title"},
		{"done without id", []string{"done"}, 2, "exactly one task id"},
		{"done with two ids", []string{"done", "1", "2"}, 2, "exactly one task id"},
		{"done with text", []string{"done", "abc"}, 2, "invalid task id"},
		{"rm with zero", []string{"rm", "0"}, 2, "invalid task id"},
		{"rm with negative", []string{"rm", "-4"}, 2, "invalid task id"},
		{"bad status", []string{"list", "-status", "bogus"}, 2, "invalid -status"},
		{"unknown list flag", []string{"list", "-x"}, 2, "not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(path, tt.args...)
			if code != tt.wantCode || stdout != "" || !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("run(%q) = (%d, %q, %q), want code %d, empty stdout, stderr containing %q",
					tt.args, code, stdout, stderr, tt.wantCode, tt.wantErrHas)
			}
		})
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("usage errors must not create the task file")
	}
}

func TestHelpAndMissingFlagValue(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"-h"}, strings.NewReader(""), &out, &errOut); code != 0 || !strings.Contains(errOut.String(), "usage: todo-cli") {
		t.Errorf("-h = (%d, %q), want code 0 and usage text", code, errOut.String())
	}
	errOut.Reset()
	if code := run([]string{"-f"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Errorf("-f without value = %d, want 2", code)
	}
}

func TestFileProblems(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		path       string
		args       []string
		wantCode   int
		wantOut    string
		wantErrHas string
	}{
		{"corrupt file on list", corrupt, []string{"list"}, 1, "", "parsing"},
		{"corrupt file on add", corrupt, []string{"add", "x"}, 1, "", "parsing"},
		{"empty file counts as no tasks", empty, []string{"list"}, 0, "no tasks\n", ""},
		{"add to an empty file", empty, []string{"add", "first"}, 0, "added #1: first\n", ""},
		{"directory does not exist", filepath.Join(dir, "missing", "t.json"), []string{"add", "x"}, 1, "", "no such file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(tt.path, tt.args...)
			if code != tt.wantCode || stdout != tt.wantOut || !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("run(%q) = (%d, %q, %q), want (%d, %q, containing %q)",
					tt.args, code, stdout, stderr, tt.wantCode, tt.wantOut, tt.wantErrHas)
			}
		})
	}

	// Failed saves must not leave temporary files behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temporary file %s", e.Name())
		}
	}
}

func TestDefaultFileIsInTheCurrentDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := run([]string{"add", "local"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr %q", code, errOut.String())
	}
	if _, err := os.Stat("todos.json"); err != nil {
		t.Errorf("todos.json was not created: %v", err)
	}
}
