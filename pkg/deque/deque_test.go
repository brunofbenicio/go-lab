package deque

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestPushPop(t *testing.T) {
	tests := []struct {
		name      string
		ops       func(d *Deque[int])
		wantSlice []int
	}{
		{"push back keeps order", func(d *Deque[int]) {
			d.PushBack(1)
			d.PushBack(2)
			d.PushBack(3)
		}, []int{1, 2, 3}},
		{"push front reverses order", func(d *Deque[int]) {
			d.PushFront(1)
			d.PushFront(2)
			d.PushFront(3)
		}, []int{3, 2, 1}},
		{"mixed ends", func(d *Deque[int]) {
			d.PushBack(2)
			d.PushFront(1)
			d.PushBack(3)
			d.PopFront()
		}, []int{2, 3}},
		{"pop back", func(d *Deque[int]) {
			d.PushBack(1)
			d.PushBack(2)
			d.PopBack()
		}, []int{1}},
		{"grows past initial capacity with wraparound", func(d *Deque[int]) {
			for i := 0; i < 3; i++ {
				d.PushBack(i)
			}
			d.PopFront()
			d.PopFront()
			for i := 3; i < 20; i++ {
				d.PushBack(i)
			}
		}, []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Deque[int]
			tt.ops(&d)
			if got := d.Slice(); !slices.Equal(got, tt.wantSlice) {
				t.Errorf("Slice() = %v, want %v", got, tt.wantSlice)
			}
			if d.Len() != len(tt.wantSlice) {
				t.Errorf("Len() = %d, want %d", d.Len(), len(tt.wantSlice))
			}
		})
	}
}

func TestEmptyAndPeek(t *testing.T) {
	var d Deque[string]
	if _, ok := d.PopFront(); ok {
		t.Error("PopFront on empty should report false")
	}
	if _, ok := d.PopBack(); ok {
		t.Error("PopBack on empty should report false")
	}
	if _, ok := d.Front(); ok {
		t.Error("Front on empty should report false")
	}
	d.PushBack("x")
	d.PushBack("y")
	if v, _ := d.Front(); v != "x" {
		t.Errorf("Front = %q, want x", v)
	}
	if v, _ := d.Back(); v != "y" {
		t.Errorf("Back = %q, want y", v)
	}
	if v, ok := d.At(1); !ok || v != "y" {
		t.Errorf("At(1) = %q, %v; want y, true", v, ok)
	}
	if _, ok := d.At(2); ok {
		t.Error("At out of range should report false")
	}
}

// TestAgainstSliceModel replays random operations on a Deque and on a plain
// slice, checking they agree after every step.
func TestAgainstSliceModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	var d Deque[int]
	var model []int
	for step := 0; step < 5000; step++ {
		switch op := rng.IntN(4); op {
		case 0:
			d.PushBack(step)
			model = append(model, step)
		case 1:
			d.PushFront(step)
			model = append([]int{step}, model...)
		case 2:
			v, ok := d.PopFront()
			if ok != (len(model) > 0) || (ok && v != model[0]) {
				t.Fatalf("step %d: PopFront = %d, %v; model %v", step, v, ok, model)
			}
			if ok {
				model = model[1:]
			}
		case 3:
			v, ok := d.PopBack()
			if ok != (len(model) > 0) || (ok && v != model[len(model)-1]) {
				t.Fatalf("step %d: PopBack = %d, %v; model %v", step, v, ok, model)
			}
			if ok {
				model = model[:len(model)-1]
			}
		}
		if !slices.Equal(d.Slice(), model) && !(len(model) == 0 && d.Len() == 0) {
			t.Fatalf("step %d: deque %v != model %v", step, d.Slice(), model)
		}
	}
}
