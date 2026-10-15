package ringbuffer

import (
	"slices"
	"sync"
	"testing"
)

func TestPushAndItems(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int
		push        []int
		wantItems   []int
		wantEvicted []int
	}{
		{"below capacity", 3, []int{1, 2}, []int{1, 2}, nil},
		{"exactly full", 3, []int{1, 2, 3}, []int{1, 2, 3}, nil},
		{"overwrites oldest", 3, []int{1, 2, 3, 4, 5}, []int{3, 4, 5}, []int{1, 2}},
		{"wraps more than once", 2, []int{1, 2, 3, 4, 5, 6, 7}, []int{6, 7}, []int{1, 2, 3, 4, 5}},
		{"capacity one", 1, []int{1, 2, 3}, []int{3}, []int{1, 2}},
		{"empty", 3, nil, []int{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New[int](tt.capacity)
			var evicted []int
			for _, v := range tt.push {
				if old, ok := b.Push(v); ok {
					evicted = append(evicted, old)
				}
			}
			if got := b.Items(); !slices.Equal(got, tt.wantItems) {
				t.Errorf("Items() = %v, want %v", got, tt.wantItems)
			}
			if !slices.Equal(evicted, tt.wantEvicted) {
				t.Errorf("evicted = %v, want %v", evicted, tt.wantEvicted)
			}
			if b.Len() != len(tt.wantItems) || b.Cap() != tt.capacity {
				t.Errorf("Len/Cap = %d/%d, want %d/%d", b.Len(), b.Cap(), len(tt.wantItems), tt.capacity)
			}
		})
	}
}

func TestLast(t *testing.T) {
	b := New[int](4)
	for i := 1; i <= 6; i++ { // keeps 3, 4, 5, 6
		b.Push(i)
	}
	tests := []struct {
		n    int
		want []int
	}{
		{0, []int{}},
		{2, []int{5, 6}},
		{4, []int{3, 4, 5, 6}},
		{10, []int{3, 4, 5, 6}},
		{-1, []int{}},
	}
	for _, tt := range tests {
		if got := b.Last(tt.n); !slices.Equal(got, tt.want) {
			t.Errorf("Last(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

func TestFullAndReset(t *testing.T) {
	b := New[string](2)
	b.Push("a")
	if b.Full() {
		t.Error("Full() should be false with 1 of 2 elements")
	}
	b.Push("b")
	if !b.Full() {
		t.Error("Full() should be true with 2 of 2 elements")
	}
	b.Reset()
	if b.Len() != 0 || len(b.Items()) != 0 {
		t.Errorf("after Reset: Len = %d, Items = %v", b.Len(), b.Items())
	}
	b.Push("c")
	if got := b.Items(); !slices.Equal(got, []string{"c"}) {
		t.Errorf("Items() after Reset+Push = %v, want [c]", got)
	}
}

func TestNewPanicsOnInvalidCapacity(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(0) should panic")
		}
	}()
	New[int](0)
}

func TestConcurrentPush(t *testing.T) {
	b := New[int](10)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				b.Push(i)
				b.Items()
			}
		}()
	}
	wg.Wait()
	if b.Len() != 10 {
		t.Errorf("Len() = %d, want 10", b.Len())
	}
}
