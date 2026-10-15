package wrap

import (
	"errors"
	"runtime"
	"slices"
	"testing"
)

// Test helpers. Every one of them calls t.Helper() first: when an assertion
// fails, the report then points at the line of the *test* that called the
// helper instead of a line inside the helper itself.
//
// They accept testing.TB (implemented by *testing.T and *testing.B) so the
// same helpers work in tests and in benchmarks.

func assertEqual[T comparable](t testing.TB, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func assertLines(t testing.TB, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func assertErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("error = %v, want %v", err, target)
	}
}

// mustWrap fails the test immediately on error, so callers can use the result
// without an error check of their own. Fatalf is for "cannot continue" cases.
func mustWrap(t testing.TB, text string, width int) []string {
	t.Helper()
	lines, err := Wrap(text, width)
	if err != nil {
		t.Fatalf("Wrap(%q, %d): %v", text, width, err)
	}
	return lines
}

func TestWrap(t *testing.T) {
	const text = "The quick brown fox jumps over the lazy dog"

	t.Run("breaks at whitespace", func(t *testing.T) {
		assertLines(t, mustWrap(t, text, 16),
			"The quick brown", "fox jumps over", "the lazy dog")
	})
	t.Run("long word stays whole", func(t *testing.T) {
		assertLines(t, mustWrap(t, "a extraordinarily b", 5),
			"a", "extraordinarily", "b")
	})
	t.Run("collapses whitespace", func(t *testing.T) {
		assertLines(t, mustWrap(t, "  a \n\t b   c ", 10), "a b c")
	})
	t.Run("counts runes, not bytes", func(t *testing.T) {
		// "façade" is 6 runes but 7 bytes; two of them (13 runes) fit in width 13.
		assertLines(t, mustWrap(t, "façade façade façade", 13), "façade façade", "façade")
	})
	t.Run("empty text has no lines", func(t *testing.T) {
		assertEqual(t, len(mustWrap(t, "   ", 10)), 0)
	})
	t.Run("invalid width", func(t *testing.T) {
		_, err := Wrap(text, 0)
		assertErrorIs(t, err, ErrWidth)
	})
}

// spyTB lets us test the helpers themselves. *testing.T cannot be told "this
// failure is expected", so we embed the testing.TB interface and override only
// the methods the helpers call, recording the calls instead of failing.
type spyTB struct {
	testing.TB // nil: any method not overridden below would panic
	failed     bool
	messages   []string
	helperCall bool
}

func (s *spyTB) Helper() { s.helperCall = true }

func (s *spyTB) Errorf(format string, args ...any) {
	s.failed = true
	s.messages = append(s.messages, format)
}

func (s *spyTB) Fatalf(format string, args ...any) {
	s.Errorf(format, args...)
	runtime.Goexit() // what the real Fatalf does: stop the calling goroutine
}

// runWith executes fn against a spyTB in its own goroutine, so a Fatalf inside
// the helper (which calls runtime.Goexit) does not stop the real test.
func runWith(fn func(testing.TB)) *spyTB {
	spy := &spyTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(spy)
	}()
	<-done
	return spy
}

func TestHelpers(t *testing.T) {
	t.Run("assertEqual passes silently", func(t *testing.T) {
		spy := runWith(func(tb testing.TB) { assertEqual(tb, 3, 3) })
		assertEqual(t, spy.failed, false)
		assertEqual(t, spy.helperCall, true)
	})
	t.Run("assertEqual reports a mismatch", func(t *testing.T) {
		spy := runWith(func(tb testing.TB) { assertEqual(tb, "a", "b") })
		assertEqual(t, spy.failed, true)
	})
	t.Run("assertLines reports a mismatch", func(t *testing.T) {
		spy := runWith(func(tb testing.TB) { assertLines(tb, []string{"x"}, "y") })
		assertEqual(t, spy.failed, true)
	})
	t.Run("mustWrap stops at the first error", func(t *testing.T) {
		reachedEnd := false
		spy := runWith(func(tb testing.TB) {
			mustWrap(tb, "text", -1)
			reachedEnd = true // never runs: Fatalf called runtime.Goexit
		})
		assertEqual(t, spy.failed, true)
		assertEqual(t, reachedEnd, false)
	})
}
