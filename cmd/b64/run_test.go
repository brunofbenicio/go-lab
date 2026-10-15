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

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantOut    string
		wantErrHas string
	}{
		{"encode", nil, "hello", 0, "aGVsbG8=\n", ""},
		{"encode sentence", nil, "Hello, World!", 0, "SGVsbG8sIFdvcmxkIQ==\n", ""},
		{"encode empty", nil, "", 0, "\n", ""},
		{"encode standard alphabet", nil, "\xfb\xff", 0, "+/8=\n", ""},
		{"encode url-safe alphabet", []string{"-url"}, "\xfb\xff", 0, "-_8=\n", ""},
		{"encode without padding", []string{"-raw"}, "hello", 0, "aGVsbG8\n", ""},
		{"encode url-safe without padding", []string{"-url", "-raw"}, "\xfb\xff", 0, "-_8\n", ""},
		{"decode", []string{"-d"}, "aGVsbG8=\n", 0, "hello", ""},
		{"decode wrapped input", []string{"-d"}, "SGVsbG8s\r\nIFdvcmxk IQ==\n", 0, "Hello, World!", ""},
		{"decode url-safe", []string{"-d", "-url"}, "-_8=", 0, "\xfb\xff", ""},
		{"decode without padding", []string{"-d", "-raw"}, "aGVsbG8", 0, "hello", ""},
		{"decode empty", []string{"-d"}, "\n", 0, "", ""},
		{"decode garbage", []string{"-d"}, "!!!", 1, "", "invalid base64"},
		{"decode truncated padding", []string{"-d"}, "aGVsbG8", 1, "", "invalid base64"},
		{"decode padded input as raw", []string{"-d", "-raw"}, "aGVsbG8=", 1, "", "invalid base64"},
		{"decode url alphabet as standard", []string{"-d"}, "-_8=", 1, "", "invalid base64"},
		{"two files", []string{"a", "b"}, "", 2, "", "at most one file"},
		{"unknown flag", []string{"-z"}, "", 2, "", "not defined"},
		{"help", []string{"-h"}, "", 0, "", "usage: b64"},
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

func TestRunFileOperand(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("plain.txt", []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("encoded.txt", []byte("aGVsbG8=\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"encode a file", []string{"plain.txt"}, 0, "aGVsbG8=\n"},
		{"decode a file", []string{"-d", "encoded.txt"}, 0, "hello"},
		{"missing file", []string{"missing.txt"}, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, _ := runCLI(t, "ignored", tt.args...)
			if code != tt.wantCode || stdout != tt.wantOut {
				t.Errorf("run(%q) = (%d, %q), want (%d, %q)", tt.args, code, stdout, tt.wantCode, tt.wantOut)
			}
		})
	}
}

func TestRoundTripAllByteValues(t *testing.T) {
	var all strings.Builder
	for i := 0; i < 256; i++ {
		all.WriteByte(byte(i))
	}
	for _, flags := range [][]string{nil, {"-url"}, {"-raw"}, {"-url", "-raw"}} {
		_, encoded, _ := runCLI(t, all.String(), flags...)
		code, decoded, stderr := runCLI(t, encoded, append([]string{"-d"}, flags...)...)
		if code != 0 || decoded != all.String() {
			t.Errorf("flags %v: round trip failed (code %d, stderr %q)", flags, code, stderr)
		}
	}
}
