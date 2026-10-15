package main

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
)

// useRandom swaps the random source for the duration of a test.
func useRandom(t *testing.T, r io.Reader) {
	t.Helper()
	old := randSource
	randSource = r
	t.Cleanup(func() { randSource = old })
}

// sequential returns a reader producing the bytes 0, 1, 2, ... up to n.
func sequential(n int) io.Reader {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return bytes.NewReader(b)
}

func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunWithKnownBytes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, "00010203-0405-4607-8809-0a0b0c0d0e0f\n"},
		{"upper case", []string{"-upper"}, "00010203-0405-4607-8809-0A0B0C0D0E0F\n"},
		{"without dashes", []string{"-no-dashes"}, "000102030405460788090a0b0c0d0e0f\n"},
		{"both options", []string{"-upper", "-no-dashes"}, "000102030405460788090A0B0C0D0E0F\n"},
		{
			"several, each from fresh bytes", []string{"-n", "2"},
			"00010203-0405-4607-8809-0a0b0c0d0e0f\n10111213-1415-4617-9819-1a1b1c1d1e1f\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useRandom(t, sequential(32))
			code, stdout, stderr := runCLI(tt.args...)
			if code != 0 || stdout != tt.want || stderr != "" {
				t.Errorf("run(%q) = (%d, %q, %q), want (0, %q, \"\")", tt.args, code, stdout, stderr, tt.want)
			}
		})
	}
}

func TestRunWithRealRandomness(t *testing.T) {
	v4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	code, stdout, stderr := runCLI("-n", "50")
	if code != 0 || stderr != "" {
		t.Fatalf("run() = (%d, %q), stderr %q", code, stdout, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 50 {
		t.Fatalf("got %d lines, want 50", len(lines))
	}
	seen := make(map[string]bool)
	for _, id := range lines {
		if !v4.MatchString(id) {
			t.Errorf("%q is not a valid version 4 UUID", id)
		}
		if seen[id] {
			t.Errorf("duplicate UUID %q", id)
		}
		seen[id] = true
	}
}

func TestRunRandomSourceFailures(t *testing.T) {
	tests := []struct {
		name   string
		source io.Reader
		args   []string
	}{
		{"too few bytes", sequential(10), nil},
		{"second UUID runs out of bytes", sequential(20), []string{"-n", "2"}},
		{"read error", iotest.ErrReader(io.ErrClosedPipe), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useRandom(t, tt.source)
			code, stdout, stderr := runCLI(tt.args...)
			if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "uuidgen:") {
				t.Errorf("run() = (%d, %q, %q), want code 1, no stdout and an error on stderr", code, stdout, stderr)
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
		{"zero count", []string{"-n", "0"}, 2, "-n must be between"},
		{"negative count", []string{"-n", "-3"}, 2, "-n must be between"},
		{"count too large", []string{"-n", "1001"}, 2, "-n must be between"},
		{"count is not a number", []string{"-n", "many"}, 2, "invalid value"},
		{"unexpected argument", []string{"extra"}, 2, "unexpected argument"},
		{"unknown flag", []string{"-x"}, 2, "not defined"},
		{"help", []string{"-h"}, 0, "usage: uuidgen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useRandom(t, sequential(16))
			code, stdout, stderr := runCLI(tt.args...)
			if code != tt.wantCode || stdout != "" || !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("run(%q) = (%d, %q, %q), want code %d, empty stdout, stderr containing %q",
					tt.args, code, stdout, stderr, tt.wantCode, tt.wantErrHas)
			}
		})
	}
}
