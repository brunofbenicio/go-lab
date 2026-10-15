package intset

import (
	"errors"
	"math/rand"
	"reflect"
	"slices"
	"testing"
	"testing/quick"
)

// config makes property tests reproducible. With a fixed seed, a failure can
// be replayed; without one, quick uses a time-based seed and a flaky property
// would fail only now and then.
func config() *quick.Config {
	return &quick.Config{MaxCount: 500, Rand: rand.New(rand.NewSource(1))}
}

// smallInts is a slice with a custom generator. By default quick draws ints
// from the whole int range, so duplicates would almost never occur and the
// Dedup properties would pass vacuously. Drawing from 0..9 forces collisions.
type smallInts []int

// Generate implements quick.Generator.
func (smallInts) Generate(r *rand.Rand, size int) reflect.Value {
	xs := make(smallInts, r.Intn(size+1))
	for i := range xs {
		xs[i] = r.Intn(10)
	}
	return reflect.ValueOf(xs)
}

// sortedInts is a sorted slice of arbitrary ints, the precondition of Merge.
type sortedInts []int

// Generate implements quick.Generator.
func (sortedInts) Generate(r *rand.Rand, size int) reflect.Value {
	xs := make(sortedInts, r.Intn(size+1))
	for i := range xs {
		xs[i] = r.Intn(100) - 50
	}
	slices.Sort(xs)
	return reflect.ValueOf(xs)
}

func check(t *testing.T, property any) {
	t.Helper()
	if err := quick.Check(property, config()); err != nil {
		t.Error(err) // the message includes the counterexample quick found
	}
}

func TestDedupProperties(t *testing.T) {
	t.Run("idempotent", func(t *testing.T) {
		check(t, func(xs smallInts) bool {
			once := Dedup(xs)
			return slices.Equal(once, Dedup(once))
		})
	})
	t.Run("result has no duplicates", func(t *testing.T) {
		check(t, func(xs smallInts) bool {
			seen := map[int]bool{}
			for _, x := range Dedup(xs) {
				if seen[x] {
					return false
				}
				seen[x] = true
			}
			return true
		})
	})
	t.Run("keeps exactly the same values", func(t *testing.T) {
		check(t, func(xs smallInts) bool {
			want := slices.Clone(xs)
			slices.Sort(want)
			want = slices.Compact(want)
			got := Dedup(xs)
			slices.Sort(got)
			return slices.Equal(got, want)
		})
	})
	t.Run("never mutates the input", func(t *testing.T) {
		check(t, func(xs smallInts) bool {
			before := slices.Clone(xs)
			Dedup(xs)
			return slices.Equal(before, xs)
		})
	})
}

func TestMergeProperties(t *testing.T) {
	t.Run("result is sorted", func(t *testing.T) {
		check(t, func(a, b sortedInts) bool { return slices.IsSorted(Merge(a, b)) })
	})
	t.Run("length is the sum", func(t *testing.T) {
		check(t, func(a, b sortedInts) bool { return len(Merge(a, b)) == len(a)+len(b) })
	})
	t.Run("commutative", func(t *testing.T) {
		check(t, func(a, b sortedInts) bool { return slices.Equal(Merge(a, b), Merge(b, a)) })
	})
}

// quick.CheckEqual compares a fast implementation against a trivially correct
// reference on random inputs (differential testing).
func TestMergeMatchesReference(t *testing.T) {
	reference := func(a, b sortedInts) []int {
		all := append(slices.Clone(a), b...)
		slices.Sort(all)
		return all
	}
	candidate := func(a, b sortedInts) []int { return Merge(a, b) }

	if err := quick.CheckEqual(candidate, reference, config()); err != nil {
		t.Error(err)
	}
}

// A property that is false must be reported with a counterexample. This test
// checks the checker: "Dedup always returns a shorter slice" is wrong for
// inputs without duplicates, and quick must find one.
func TestQuickFindsCounterexamples(t *testing.T) {
	alwaysShrinks := func(xs smallInts) bool { return len(Dedup(xs)) < len(xs) }

	err := quick.Check(alwaysShrinks, config())

	var ce *quick.CheckError
	if !errors.As(err, &ce) {
		t.Fatalf("quick.Check error = %v, want a *quick.CheckError", err)
	}
	if got := ce.In[0].(smallInts); len(Dedup(got)) < len(got) {
		t.Errorf("reported input %v is not a counterexample", got)
	}
}
