package report

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update is the conventional flag for golden-file tests:
//
//	go test -run TestRender -update
//
// rewrites the expected files from the current output. Review the diff
// (git diff testdata/) before committing: that review is the real test.
var update = flag.Bool("update", false, "rewrite the golden files with the current output")

// assertGolden compares got with testdata/<test name>.golden. The file name is
// derived from t.Name(), so every subtest owns its golden file automatically.
func assertGolden(t *testing.T, got []byte) {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s (run with -update to accept)\n--- got ---\n%s--- want ---\n%s",
			path, got, want)
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		rows []Row
	}{
		{"basic", []Row{
			{"Keyboard", 2, 4999},
			{"Cable", 10, 350},
			{"Monitor", 1, 129900},
		}},
		{"empty", nil},
		{"unicode names align by rune", []Row{
			{"Café", 3, 1250},
			{"Crème brûlée", 12, 475},
		}},
		{"negative amount", []Row{
			{"Refund", 1, -2500},
			{"Item", 1, 1000},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, tt.rows); err != nil {
				t.Fatalf("Render: %v", err)
			}
			assertGolden(t, buf.Bytes())
		})
	}
}

// Golden files are for large or visual output. Small invariants are better
// asserted directly, where the intent is visible in the test itself.
func TestMoney(t *testing.T) {
	for in, want := range map[int64]string{0: "0.00", 5: "0.05", 1999: "19.99", -250: "-2.50"} {
		if got := money(in); got != want {
			t.Errorf("money(%d) = %q, want %q", in, got, want)
		}
	}
}
