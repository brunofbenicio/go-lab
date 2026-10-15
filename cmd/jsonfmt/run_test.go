package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunFormatting(t *testing.T) {
	const nested = `{"b":1,"a":[1,2,{"c":null}]}`
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			name: "pretty by default and keeps key order", stdin: nested,
			want: "{\n  \"b\": 1,\n  \"a\": [\n    1,\n    2,\n    {\n      \"c\": null\n    }\n  ]\n}\n",
		},
		{"custom indent", []string{"-indent", "4"}, `{"a":1}`, "{\n    \"a\": 1\n}\n"},
		{"zero indent", []string{"-indent", "0"}, `[1,2]`, "[\n1,\n2\n]\n"},
		{"minify", []string{"-m"}, "{\n  \"a\": [1, 2],\n  \"b\": \"x y\"\n}\n", `{"a":[1,2],"b":"x y"}` + "\n"},
		{"minify keeps spaces inside strings", []string{"-m"}, `{ "k" : "a  b" }`, `{"k":"a  b"}` + "\n"},
		{"empty object", nil, `{}`, "{}\n"},
		{"empty array", nil, "[ ]", "[]\n"},
		{"scalar string", nil, `"hi"`, `"hi"` + "\n"},
		{"scalar number with whitespace", nil, "  42 \n\n", "42\n"},
		{"unicode is preserved", nil, `{"k":"ção"}`, "{\n  \"k\": \"ção\"\n}\n"},
		{"explicit dash reads stdin", []string{"-"}, `[true]`, "[\n  true\n]\n"},
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

func TestRunErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("ok.json", []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantOut    string
		wantErrHas string
	}{
		{"file argument", []string{"ok.json"}, "ignored", 0, "{\n  \"a\": 1\n}\n", ""},
		{"invalid JSON", nil, `{"a":}`, 1, "", "invalid JSON"},
		{"trailing garbage", nil, `{} x`, 1, "", "invalid JSON"},
		{"truncated JSON", []string{"-m"}, `{"a":[1,`, 1, "", "invalid JSON"},
		{"empty input", nil, "  \n", 1, "", "empty input"},
		{"missing file", []string{"nope.json"}, "", 1, "", "no such file"},
		{"two files", []string{"ok.json", "ok.json"}, "", 2, "", "at most one file"},
		{"negative indent", []string{"-indent", "-1"}, "{}", 2, "", "-indent must be"},
		{"huge indent", []string{"-indent", "99"}, "{}", 2, "", "-indent must be"},
		{"unknown flag", []string{"-x"}, "{}", 2, "", "not defined"},
		{"help", []string{"-h"}, "", 0, "", "usage: jsonfmt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.stdin, tt.args...)
			if code != tt.wantCode || stdout != tt.wantOut {
				t.Errorf("run(%q) = (%d, %q), want (%d, %q)", tt.args, code, stdout, tt.wantCode, tt.wantOut)
			}
			if !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.wantErrHas)
			}
		})
	}
}
