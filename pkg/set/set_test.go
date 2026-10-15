package set

import (
	"slices"
	"testing"
)

func sorted(s Set[int]) []int {
	out := s.Slice()
	slices.Sort(out)
	return out
}

func TestOperations(t *testing.T) {
	a, b := New(1, 2, 3, 4), New(3, 4, 5)
	tests := []struct {
		name string
		got  Set[int]
		want []int
	}{
		{"union", a.Union(b), []int{1, 2, 3, 4, 5}},
		{"intersect", a.Intersect(b), []int{3, 4}},
		{"difference a-b", a.Difference(b), []int{1, 2}},
		{"difference b-a", b.Difference(a), []int{5}},
		{"symmetric difference", a.SymmetricDifference(b), []int{1, 2, 5}},
		{"union with empty", a.Union(New[int]()), []int{1, 2, 3, 4}},
		{"intersect with empty", a.Intersect(New[int]()), []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sorted(tt.got); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
	if a.Len() != 4 || b.Len() != 3 {
		t.Error("operations must not modify their operands")
	}
}

func TestAddRemoveHas(t *testing.T) {
	s := New("a", "b", "a")
	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (duplicates collapse)", s.Len())
	}
	s.Remove("a", "zzz")
	if s.Has("a") || !s.Has("b") {
		t.Errorf("after Remove: Has(a)=%v Has(b)=%v", s.Has("a"), s.Has("b"))
	}
	s.Add("c", "d")
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3", s.Len())
	}
}

func TestRelations(t *testing.T) {
	tests := []struct {
		name         string
		a, b         Set[int]
		subset, same bool
	}{
		{"equal sets", New(1, 2), New(2, 1), true, true},
		{"proper subset", New(1), New(1, 2), true, false},
		{"superset", New(1, 2), New(1), false, false},
		{"disjoint", New(1), New(2), false, false},
		{"empty is subset of anything", New[int](), New(1), true, false},
		{"both empty", New[int](), New[int](), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.IsSubset(tt.b); got != tt.subset {
				t.Errorf("IsSubset = %v, want %v", got, tt.subset)
			}
			if got := tt.a.Equal(tt.b); got != tt.same {
				t.Errorf("Equal = %v, want %v", got, tt.same)
			}
		})
	}
}

func TestCloneIsIndependent(t *testing.T) {
	a := New(1, 2)
	c := a.Clone()
	c.Add(3)
	a.Remove(1)
	if a.Equal(c) || c.Has(3) == a.Has(3) || !c.Has(1) {
		t.Errorf("clone should be independent: a=%v c=%v", sorted(a), sorted(c))
	}
}
