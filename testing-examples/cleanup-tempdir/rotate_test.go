package rotate

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// newWriter is a setup helper. t.TempDir() gives the test a private directory
// that Go deletes automatically, and t.Cleanup registers the Close call so the
// caller never has to remember it (or write a defer).
func newWriter(t *testing.T, maxSize int64, keep int) (*Writer, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := New(dir, "app.log", maxSize, keep)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return w, dir
}

func mustWrite(t *testing.T, w *Writer, s string) {
	t.Helper()
	if n, err := w.Write([]byte(s)); err != nil || n != len(s) {
		t.Fatalf("Write(%q) = %d, %v", s, n, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names // os.ReadDir already sorts by name
}

func TestNoRotationBelowLimit(t *testing.T) {
	w, dir := newWriter(t, 10, 2)
	mustWrite(t, w, "abcde")
	mustWrite(t, w, "fghij") // exactly 10 bytes: still fits

	if got := listDir(t, dir); !slices.Equal(got, []string{"app.log"}) {
		t.Errorf("files = %v, want only app.log", got)
	}
	if got := readFile(t, filepath.Join(dir, "app.log")); got != "abcdefghij" {
		t.Errorf("app.log = %q", got)
	}
}

func TestRotationKeepsBackups(t *testing.T) {
	w, dir := newWriter(t, 5, 2)
	for _, s := range []string{"11111", "22222", "33333", "44444"} {
		mustWrite(t, w, s) // each write fills the file, forcing a rotation
	}

	want := map[string]string{
		"app.log":   "44444",
		"app.log.1": "33333",
		"app.log.2": "22222", // "11111" fell off the end: keep == 2
	}
	if got := listDir(t, dir); len(got) != len(want) {
		t.Fatalf("files = %v, want %d files", got, len(want))
	}
	for name, content := range want {
		if got := readFile(t, filepath.Join(dir, name)); got != content {
			t.Errorf("%s = %q, want %q", name, got, content)
		}
	}
}

func TestKeepZeroDiscardsOldData(t *testing.T) {
	w, dir := newWriter(t, 4, 0)
	mustWrite(t, w, "aaaa")
	mustWrite(t, w, "bbbb")

	if got := listDir(t, dir); !slices.Equal(got, []string{"app.log"}) {
		t.Errorf("files = %v, want only app.log", got)
	}
	if got := readFile(t, filepath.Join(dir, "app.log")); got != "bbbb" {
		t.Errorf("app.log = %q, want bbbb", got)
	}
}

func TestReopenAppendsToExistingFile(t *testing.T) {
	w, dir := newWriter(t, 100, 1)
	mustWrite(t, w, "first;")
	if err := w.Close(); err != nil { // explicit Close; the Cleanup Close is a no-op
		t.Fatalf("Close: %v", err)
	}

	w2, err := New(dir, "app.log", 100, 1)
	if err != nil {
		t.Fatalf("New (reopen): %v", err)
	}
	defer w2.Close()
	mustWrite(t, w2, "second;")

	if got := readFile(t, filepath.Join(dir, "app.log")); got != "first;second;" {
		t.Errorf("app.log = %q, want first;second;", got)
	}
}

func TestWriteAfterClose(t *testing.T) {
	w, _ := newWriter(t, 10, 1)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := w.Write([]byte("x")); err == nil {
		t.Error("Write after Close succeeded, want an error")
	}
}

func TestNewInMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does", "not", "exist")
	if _, err := New(missing, "app.log", 10, 1); err == nil {
		t.Error("New in a missing directory succeeded, want an error")
	}
}

// Cleanup functions run in last-in, first-out order, like defer, and always
// run when the (sub)test ends, even if it failed or called t.Fatal.
func TestCleanupOrderAndTempDirRemoval(t *testing.T) {
	var order []string
	var dir string

	t.Run("inner", func(t *testing.T) {
		dir = t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { order = append(order, "registered first") })
		t.Cleanup(func() { order = append(order, "registered second") })
	})

	want := []string{"registered second", "registered first"}
	if !slices.Equal(order, want) {
		t.Errorf("cleanup order = %v, want %v", order, want)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("TempDir %s still exists after the subtest (err = %v)", dir, err)
	}
}
