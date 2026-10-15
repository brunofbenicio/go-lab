package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// runCLI runs the command and returns its exit status, stdout and stderr.
func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunStdin(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{"default columns share one width", nil, "hello world\nfoo\n", " 2  3 16\n"},
		{"lines only", []string{"-l"}, "hello world\nfoo\n", "2\n"},
		{"words only", []string{"-w"}, "hello world\nfoo\n", "3\n"},
		{"bytes only", []string{"-c"}, "hello world\nfoo\n", "16\n"},
		{"characters differ from bytes", []string{"-m"}, "héllo\n", "6\n"},
		{"bytes of multibyte text", []string{"-c"}, "héllo\n", "7\n"},
		{"all columns in fixed order", []string{"-c", "-m", "-w", "-l"}, "hello world\nfoo\n", " 2  3 16 16\n"},
		{"empty input", nil, "", "0 0 0\n"},
		{"no trailing newline", nil, "a b", "0 2 3\n"},
		{"mixed whitespace", nil, "  a \t b\n\n", "2 2 9\n"},
		{"explicit dash shows its name", []string{"-"}, "x\n", "1 1 2 -\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.stdin, tt.args...)
			if code != 0 || stdout != tt.want || stderr != "" {
				t.Errorf("run(%q) = (%d, %q, %q), want (0, %q, \"\")", tt.args, code, stdout, stderr, tt.want)
			}
		})
	}
}

func TestRunFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("a.txt", []byte("one two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("b.txt", []byte("three\nfour five six\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    string
		wantErrHas string
	}{
		{"single file", []string{"a.txt"}, 0, "1 2 8 a.txt\n", ""},
		{
			"several files get a total", []string{"a.txt", "b.txt"}, 0,
			" 1  2  8 a.txt\n 2  4 20 b.txt\n 3  6 28 total\n", "",
		},
		{"only lines of two files", []string{"-l", "a.txt", "b.txt"}, 0, "1 a.txt\n2 b.txt\n3 total\n", ""},
		{"missing file is reported", []string{"a.txt", "missing.txt"}, 1, "1 2 8 a.txt\n", "missing.txt"},
		{"only missing files", []string{"missing.txt"}, 1, "", "no such file"},
		{"directory is an error", []string{"."}, 1, "", "is a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, "", tt.args...)
			if code != tt.wantCode || stdout != tt.wantOut {
				t.Errorf("run(%q) = (%d, %q), want (%d, %q)", tt.args, code, stdout, tt.wantCode, tt.wantOut)
			}
			if !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.wantErrHas)
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantErrHas string
	}{
		{"unknown flag", []string{"-z"}, 2, "not defined"},
		{"help", []string{"-h"}, 0, "usage: wc-lite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, "", tt.args...)
			if code != tt.wantCode || stdout != "" || !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("run(%q) = (%d, %q, %q), want code %d, empty stdout, stderr containing %q",
					tt.args, code, stdout, stderr, tt.wantCode, tt.wantErrHas)
			}
		})
	}
}
