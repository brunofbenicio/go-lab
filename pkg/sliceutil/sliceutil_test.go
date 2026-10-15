package sliceutil

import (
	"reflect"
	"slices"
	"strconv"
	"testing"
)

func isEven(n int) bool { return n%2 == 0 }

func TestMapFilterReduce(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}

	if got, want := Map(nums, strconv.Itoa), []string{"1", "2", "3", "4", "5"}; !slices.Equal(got, want) {
		t.Errorf("Map = %v, want %v", got, want)
	}
	if got := Map([]int(nil), strconv.Itoa); len(got) != 0 {
		t.Errorf("Map(nil) = %v, want empty", got)
	}

	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"keeps evens", nums, []int{2, 4}},
		{"none match", []int{1, 3}, []int{}},
		{"empty", nil, []int{}},
	}
	for _, tt := range tests {
		if got := Filter(tt.in, isEven); !slices.Equal(got, tt.want) {
			t.Errorf("Filter(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}

	if got := Reduce(nums, 0, func(acc, v int) int { return acc + v }); got != 15 {
		t.Errorf("Reduce sum = %d, want 15", got)
	}
	// The accumulator may have a different type from the elements.
	if got := Reduce(nums, "", func(acc string, v int) string { return acc + strconv.Itoa(v) }); got != "12345" {
		t.Errorf("Reduce concat = %q, want 12345", got)
	}
	if got := Reduce([]int(nil), 42, func(acc, v int) int { return acc + v }); got != 42 {
		t.Errorf("Reduce(empty) = %d, want the initial value 42", got)
	}
}

func TestGroupBy(t *testing.T) {
	words := []string{"apple", "avocado", "banana", "blueberry", "cherry"}
	got := GroupBy(words, func(s string) byte { return s[0] })
	want := map[byte][]string{
		'a': {"apple", "avocado"},
		'b': {"banana", "blueberry"},
		'c': {"cherry"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupBy = %v, want %v", got, want)
	}
	if got := GroupBy([]int(nil), func(n int) int { return n }); len(got) != 0 {
		t.Errorf("GroupBy(nil) = %v, want empty map", got)
	}
}

func TestChunk(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		size int
		want [][]int
	}{
		{"even split", []int{1, 2, 3, 4}, 2, [][]int{{1, 2}, {3, 4}}},
		{"short last chunk", []int{1, 2, 3, 4, 5}, 2, [][]int{{1, 2}, {3, 4}, {5}}},
		{"size larger than input", []int{1, 2}, 10, [][]int{{1, 2}}},
		{"size one", []int{1, 2, 3}, 1, [][]int{{1}, {2}, {3}}},
		{"empty", nil, 3, [][]int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Chunk(tt.in, tt.size); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Chunk = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("appending to a chunk does not clobber the next", func(t *testing.T) {
		chunks := Chunk([]int{1, 2, 3, 4}, 2)
		_ = append(chunks[0], 99)
		if chunks[1][0] != 3 {
			t.Errorf("neighbouring chunk was overwritten: %v", chunks)
		}
	})

	t.Run("non-positive size panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Chunk(s, 0) should panic")
			}
		}()
		Chunk([]int{1}, 0)
	})
}

func TestUnique(t *testing.T) {
	if got, want := Unique([]int{3, 1, 3, 2, 1}), []int{3, 1, 2}; !slices.Equal(got, want) {
		t.Errorf("Unique = %v, want %v", got, want)
	}
	if got := Unique([]string(nil)); len(got) != 0 {
		t.Errorf("Unique(nil) = %v, want empty", got)
	}

	type user struct {
		ID   int
		Name string
	}
	users := []user{{1, "ana"}, {2, "bia"}, {1, "ana (dup)"}, {3, "caio"}}
	got := UniqueBy(users, func(u user) int { return u.ID })
	if want := []user{{1, "ana"}, {2, "bia"}, {3, "caio"}}; !slices.Equal(got, want) {
		t.Errorf("UniqueBy = %v, want %v", got, want)
	}
}

func TestPartitionAndFlatten(t *testing.T) {
	even, odd := Partition([]int{1, 2, 3, 4, 5}, isEven)
	if !slices.Equal(even, []int{2, 4}) || !slices.Equal(odd, []int{1, 3, 5}) {
		t.Errorf("Partition = %v, %v", even, odd)
	}
	yes, no := Partition([]int(nil), isEven)
	if len(yes) != 0 || len(no) != 0 {
		t.Errorf("Partition(nil) = %v, %v; want both empty", yes, no)
	}

	if got, want := Flatten([][]int{{1, 2}, nil, {3}, {}}), []int{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("Flatten = %v, want %v", got, want)
	}
}

func TestInputIsNeverModified(t *testing.T) {
	in := []int{3, 1, 2, 1}
	snapshot := slices.Clone(in)
	Map(in, func(n int) int { return n * 2 })
	Filter(in, isEven)
	Unique(in)
	Partition(in, isEven)
	Chunk(in, 3)
	if !slices.Equal(in, snapshot) {
		t.Errorf("input changed: %v, want %v", in, snapshot)
	}
}
