package priorityqueue

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"
)

func drain[T any](q *Queue[T]) []T {
	var out []T
	for {
		v, ok := q.Pop()
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func TestOrdering(t *testing.T) {
	tests := []struct {
		name    string
		less    func(a, b int) bool
		initial []int
		push    []int
		want    []int
	}{
		{"min-heap", func(a, b int) bool { return a < b }, nil, []int{5, 1, 4, 2, 3}, []int{1, 2, 3, 4, 5}},
		{"max-heap", func(a, b int) bool { return a > b }, nil, []int{5, 1, 4, 2, 3}, []int{5, 4, 3, 2, 1}},
		{"initial items are heapified", func(a, b int) bool { return a < b }, []int{9, 3, 7}, []int{1}, []int{1, 3, 7, 9}},
		{"duplicates", func(a, b int) bool { return a < b }, nil, []int{2, 2, 1, 1}, []int{1, 1, 2, 2}},
		{"empty", func(a, b int) bool { return a < b }, nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := New(tt.less, tt.initial...)
			for _, v := range tt.push {
				q.Push(v)
			}
			if got := drain(q); !slices.Equal(got, tt.want) {
				t.Errorf("drained %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStructsPeekAndLen(t *testing.T) {
	type task struct {
		name     string
		priority int
	}
	q := New(func(a, b task) bool { return a.priority > b.priority })
	q.Push(task{"low", 1})
	q.Push(task{"urgent", 10})
	q.Push(task{"normal", 5})

	if top, _ := q.Peek(); top.name != "urgent" || q.Len() != 3 {
		t.Fatalf("Peek = %v, Len = %d; want urgent, 3", top, q.Len())
	}
	var names []string
	for _, tk := range drain(q) {
		names = append(names, tk.name)
	}
	if want := []string{"urgent", "normal", "low"}; !slices.Equal(names, want) {
		t.Errorf("order = %v, want %v", names, want)
	}
	if _, ok := q.Peek(); ok {
		t.Error("Peek on an empty queue should report false")
	}
}

func TestNewCopiesInitialSlice(t *testing.T) {
	src := []int{3, 1, 2}
	q := New(func(a, b int) bool { return a < b }, src...)
	q.Pop()
	if !slices.Equal(src, []int{3, 1, 2}) {
		t.Errorf("input slice was modified: %v", src)
	}
}

func TestHeapSortMatchesSort(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	input := make([]int, 500)
	for i := range input {
		input[i] = rng.IntN(100)
	}
	q := New(cmp.Less[int])
	for _, v := range input {
		q.Push(v)
	}
	want := slices.Sorted(slices.Values(input))
	if got := drain(q); !slices.Equal(got, want) {
		t.Error("popping everything should yield the sorted input")
	}
}
