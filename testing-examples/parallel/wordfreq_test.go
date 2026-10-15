package wordfreq

import (
	"reflect"
	"sync/atomic"
	"testing"
)

// Calling t.Parallel() pauses a test until all the serial tests of the package
// have run, then runs it alongside the other parallel tests (bounded by
// -parallel, which defaults to GOMAXPROCS).
func TestCountParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want map[string]int
	}{
		{"empty", "", map[string]int{}},
		{"case folding", "Go go GO", map[string]int{"go": 3}},
		{"punctuation", "one, two; two! (three)", map[string]int{"one": 1, "two": 2, "three": 1}},
		{"digits count as word characters", "go1 go2 go1", map[string]int{"go1": 2, "go2": 1}},
		{"unicode letters", "Élan élan ÉLAN", map[string]int{"élan": 3}},
	}

	// A counter that lets the parent check that every parallel subtest ran.
	var ran atomic.Int32

	// The parent's cleanup only runs after all of its subtests finish,
	// including the parallel ones that were paused until the loop ended.
	t.Cleanup(func() {
		if got := int(ran.Load()); got != len(tests) {
			t.Errorf("%d parallel subtests ran, want %d", got, len(tests))
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Since Go 1.22 each iteration has its own tt, so no
			// "tt := tt" copy is needed here.
			ran.Add(1)

			if got := Count(tt.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Count(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestTopParallel(t *testing.T) {
	t.Parallel()

	// Read-only data shared by parallel subtests is safe. Never share
	// something that a subtest writes to without synchronization.
	counts := Count("b a c a b a d")

	tests := []struct {
		name string
		n    int
		want []WordCount
	}{
		{"top two", 2, []WordCount{{"a", 3}, {"b", 2}}},
		{"ties are alphabetical", 4, []WordCount{{"a", 3}, {"b", 2}, {"c", 1}, {"d", 1}}},
		{"n larger than the vocabulary", 99, []WordCount{{"a", 3}, {"b", 2}, {"c", 1}, {"d", 1}}},
		{"zero", 0, []WordCount{}},
		{"negative", -1, []WordCount{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Top(counts, tt.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Top(n=%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}

// TestGrouped shows how to run something after a set of parallel subtests:
// wrapping them in a serial t.Run makes that call return only once every
// parallel subtest inside it has finished.
func TestGrouped(t *testing.T) {
	var done atomic.Int32

	t.Run("group", func(t *testing.T) {
		for _, text := range []string{"a b", "c d e", "f"} {
			t.Run(text, func(t *testing.T) {
				t.Parallel()
				_ = Count(text)
				done.Add(1)
			})
		}
	})

	if got := done.Load(); got != 3 {
		t.Errorf("%d subtests done after the group, want 3", got)
	}
}
