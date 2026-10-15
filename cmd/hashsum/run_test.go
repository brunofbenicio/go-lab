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

const (
	md5Abc    = "900150983cd24fb0d6963f7d28e17f72"
	sha1Abc   = "a9993e364706816aba3e25717850c26c9cd0d89d"
	sha256Abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	md5Empty  = "d41d8cd98f00b204e9800998ecf8427e"
	sha256Nil = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func TestRunStdin(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{"sha256 is the default", nil, "abc", sha256Abc + "  -\n"},
		{"explicit sha256", []string{"-a", "sha256"}, "abc", sha256Abc + "  -\n"},
		{"sha1", []string{"-a", "sha1"}, "abc", sha1Abc + "  -\n"},
		{"md5", []string{"-a", "md5"}, "abc", md5Abc + "  -\n"},
		{"algorithm name is case-insensitive", []string{"-a", "MD5"}, "abc", md5Abc + "  -\n"},
		{"empty input md5", []string{"-a", "md5"}, "", md5Empty + "  -\n"},
		{"empty input sha256", nil, "", sha256Nil + "  -\n"},
		{"dash operand", []string{"-"}, "abc", sha256Abc + "  -\n"},
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
	for name, content := range map[string]string{"abc.txt": "abc", "empty.txt": ""} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantOut    string
		wantErrHas string
	}{
		{"one file", []string{"abc.txt"}, "", 0, sha256Abc + "  abc.txt\n", ""},
		{
			"several files in order", []string{"-a", "md5", "abc.txt", "empty.txt"}, "", 0,
			md5Abc + "  abc.txt\n" + md5Empty + "  empty.txt\n", "",
		},
		{
			"files and stdin mixed", []string{"-a", "sha1", "-", "abc.txt"}, "abc", 0,
			sha1Abc + "  -\n" + sha1Abc + "  abc.txt\n", "",
		},
		{"missing file keeps going", []string{"nope.txt", "abc.txt"}, "", 1, sha256Abc + "  abc.txt\n", "nope.txt"},
		{"directory is an error", []string{"."}, "", 1, "", "is a directory"},
		{"unknown algorithm", []string{"-a", "crc32", "abc.txt"}, "", 2, "", "unknown algorithm"},
		{"unknown flag", []string{"-q"}, "", 2, "", "not defined"},
		{"help", []string{"-h"}, "", 0, "", "usage: hashsum"},
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

// Large inputs go through io.Copy in several chunks; the digest must not change.
func TestRunLargeInput(t *testing.T) {
	big := strings.Repeat("0123456789abcdef", 1<<16) // 1 MiB
	code1, out1, _ := runCLI(t, big)
	code2, out2, _ := runCLI(t, big)
	if code1 != 0 || code2 != 0 || out1 != out2 || len(out1) != len(sha256Abc)+len("  -\n") {
		t.Errorf("unstable or malformed output: %q vs %q", out1, out2)
	}
}
