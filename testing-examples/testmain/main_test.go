package kvconf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureDir is shared by every test in the package. It is created once in
// TestMain instead of once per test.
var fixtureDir string

// fixtures are the files TestMain writes into fixtureDir.
var fixtures = map[string]string{
	"app.conf": `# application settings
name = demo
port=8080

  debug =  true
name = demo-v2
`,
	"broken.conf": "name = ok\nthis line has no equals sign\n",
	"empty.conf":  "",
}

// TestMain runs once per package, before any test. It is the place for
// expensive, shared setup and the matching teardown.
//
// os.Exit does not run deferred functions, so the real work lives in
// runTests, whose defers do run, and TestMain only passes its result to os.Exit.
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	// There is no *testing.T here, so t.TempDir() is not available:
	// create the directory by hand and remove it when done.
	dir, err := os.MkdirTemp("", "kvconf-fixtures-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	for name, content := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "setup:", err)
			return 1
		}
	}
	fixtureDir = dir

	return m.Run() // runs every Test, Benchmark, Fuzz and Example
}

func TestLoadFixture(t *testing.T) {
	conf, err := Load(filepath.Join(fixtureDir, "app.conf"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{"name": "demo-v2", "port": "8080", "debug": "true"}
	if len(conf) != len(want) {
		t.Fatalf("Load = %v, want %v", conf, want)
	}
	for k, v := range want {
		if conf[k] != v {
			t.Errorf("conf[%q] = %q, want %q", k, conf[k], v)
		}
	}
}

func TestLoadEmptyFile(t *testing.T) {
	conf, err := Load(filepath.Join(fixtureDir, "empty.conf"))
	if err != nil || len(conf) != 0 {
		t.Errorf("Load(empty) = %v, %v; want empty map, nil", conf, err)
	}
}

func TestLoadBrokenFile(t *testing.T) {
	_, err := Load(filepath.Join(fixtureDir, "broken.conf"))
	if !errors.Is(err, ErrSyntax) {
		t.Fatalf("Load(broken) error = %v, want ErrSyntax", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(fixtureDir, "nope.conf"))
	if !os.IsNotExist(err) {
		t.Errorf("Load(missing) error = %v, want a not-exist error", err)
	}
}

// Tests may also create their own files on top of the shared fixtures:
// t.TempDir() is still the right tool for data that belongs to one test.
func TestLoadFileCreatedByTest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.conf")
	if err := os.WriteFile(path, []byte("k = v"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf, err := Load(path)
	if err != nil || conf["k"] != "v" {
		t.Errorf("Load = %v, %v; want k=v", conf, err)
	}
}

// Tests that need no shared files simply ignore the fixtures.
func TestParseValueMayContainEquals(t *testing.T) {
	conf, err := Parse(strings.NewReader("dsn = host=example.com port=5432\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := conf["dsn"]; got != "host=example.com port=5432" {
		t.Errorf("dsn = %q", got)
	}
}
