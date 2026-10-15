package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execute calls run with in-memory streams and returns what it produced.
func execute(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

const sample = "Apple pie\nbanana split\ncherry tart\napple crumble\n"

func TestRunOnStdin(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{"plain match", []string{"apple"}, 0, "apple crumble\n"},
		{"ignore case", []string{"-i", "apple"}, 0, "Apple pie\napple crumble\n"},
		{"line numbers", []string{"-n", "-i", "apple"}, 0, "1:Apple pie\n4:apple crumble\n"},
		{"count", []string{"-c", "-i", "apple"}, 0, "2\n"},
		{"count without matches", []string{"-c", "zzz"}, 1, "0\n"},
		{"regular expression", []string{`^b.*t$`}, 0, "banana split\n"},
		{"no match", []string{"zzz"}, 1, ""},
		{"dash reads stdin explicitly", []string{"cherry", "-"}, 0, "cherry tart\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := execute(t, sample, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %q)", code, tt.wantCode, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"missing pattern", nil, "missing PATTERN"},
		{"invalid regexp", []string{"("}, "invalid pattern"},
		{"unknown flag", []string{"-z", "x"}, "flag provided but not defined: -z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := execute(t, "", tt.args...)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	code, _, stderr := execute(t, "", "-h")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "usage: minigrep") || !strings.Contains(stderr, "-i") {
		t.Errorf("stderr = %q, want the usage text", stderr)
	}
}

func TestRunWithFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	fruits := write("fruits.txt", sample)
	veggies := write("veggies.txt", "carrot\napple-green pepper\n")

	t.Run("one file has no prefix", func(t *testing.T) {
		code, stdout, _ := execute(t, "", "-n", "cherry", fruits)
		if code != 0 || stdout != "3:cherry tart\n" {
			t.Errorf("got code=%d stdout=%q", code, stdout)
		}
	})

	t.Run("several files are prefixed with their names", func(t *testing.T) {
		code, stdout, _ := execute(t, "", "apple", fruits, veggies)
		want := fruits + ":apple crumble\n" + veggies + ":apple-green pepper\n"
		if code != 0 || stdout != want {
			t.Errorf("got code=%d stdout=%q, want %q", code, stdout, want)
		}
	})

	t.Run("count per file", func(t *testing.T) {
		_, stdout, _ := execute(t, "", "-c", "-i", "apple", fruits, veggies)
		want := fruits + ":2\n" + veggies + ":1\n"
		if stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("a missing file fails but the others are still searched", func(t *testing.T) {
		missing := filepath.Join(dir, "missing.txt")
		code, stdout, stderr := execute(t, "", "carrot", missing, veggies)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stdout, "carrot") {
			t.Errorf("stdout = %q, want the match from the existing file", stdout)
		}
		if !strings.Contains(stderr, "missing.txt") {
			t.Errorf("stderr = %q, want it to name the missing file", stderr)
		}
	})
}
