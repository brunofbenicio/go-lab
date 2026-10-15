package lru

import (
	"errors"
	"reflect"
	"testing"
)

func newCache(t *testing.T, capacity int, onEvict func(string, int)) *Cache[string, int] {
	t.Helper()
	c, err := New(capacity, onEvict)
	if err != nil {
		t.Fatalf("New(%d): %v", capacity, err)
	}
	return c
}

func TestNewRejectsBadCapacity(t *testing.T) {
	for _, capacity := range []int{0, -3} {
		if _, err := New[string, int](capacity, nil); !errors.Is(err, ErrCapacity) {
			t.Errorf("New(%d) error = %v, want ErrCapacity", capacity, err)
		}
	}
}

func TestPutGetAndEviction(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		steps    func(c *Cache[string, int])
		wantKeys []string
	}{
		{
			name:     "fills without eviction",
			capacity: 3,
			steps:    func(c *Cache[string, int]) { c.Put("a", 1); c.Put("b", 2) },
			wantKeys: []string{"b", "a"},
		},
		{
			name:     "evicts the oldest",
			capacity: 2,
			steps:    func(c *Cache[string, int]) { c.Put("a", 1); c.Put("b", 2); c.Put("c", 3) },
			wantKeys: []string{"c", "b"},
		},
		{
			name:     "get refreshes recency",
			capacity: 2,
			steps: func(c *Cache[string, int]) {
				c.Put("a", 1)
				c.Put("b", 2)
				c.Get("a")
				c.Put("c", 3) // evicts b, not a
			},
			wantKeys: []string{"c", "a"},
		},
		{
			name:     "peek does not refresh",
			capacity: 2,
			steps: func(c *Cache[string, int]) {
				c.Put("a", 1)
				c.Put("b", 2)
				c.Peek("a")
				c.Put("c", 3) // evicts a
			},
			wantKeys: []string{"c", "b"},
		},
		{
			name:     "updating a key refreshes it without eviction",
			capacity: 2,
			steps: func(c *Cache[string, int]) {
				c.Put("a", 1)
				c.Put("b", 2)
				c.Put("a", 10)
			},
			wantKeys: []string{"a", "b"},
		},
		{
			name:     "capacity one",
			capacity: 1,
			steps:    func(c *Cache[string, int]) { c.Put("a", 1); c.Put("b", 2) },
			wantKeys: []string{"b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCache(t, tt.capacity, nil)
			tt.steps(c)
			if got := c.Keys(); !reflect.DeepEqual(got, tt.wantKeys) {
				t.Errorf("Keys() = %v, want %v", got, tt.wantKeys)
			}
			if c.Len() != len(tt.wantKeys) {
				t.Errorf("Len() = %d, want %d", c.Len(), len(tt.wantKeys))
			}
		})
	}
}

func TestValues(t *testing.T) {
	c := newCache(t, 2, nil)
	c.Put("a", 1)
	c.Put("a", 2)
	if v, ok := c.Get("a"); !ok || v != 2 {
		t.Errorf("Get(a) = (%d, %v), want (2, true)", v, ok)
	}
	if v, ok := c.Get("missing"); ok || v != 0 {
		t.Errorf("Get(missing) = (%d, %v), want (0, false)", v, ok)
	}
	if v, ok := c.Peek("a"); !ok || v != 2 {
		t.Errorf("Peek(a) = (%d, %v), want (2, true)", v, ok)
	}
	if _, ok := c.Peek("missing"); ok {
		t.Error("Peek(missing) reported a hit")
	}
}

func TestDelete(t *testing.T) {
	c := newCache(t, 3, nil)
	c.Put("a", 1)
	c.Put("b", 2)
	if !c.Delete("a") {
		t.Error("Delete(a) = false, want true")
	}
	if c.Delete("a") {
		t.Error("second Delete(a) = true, want false")
	}
	if _, ok := c.Get("a"); ok || c.Len() != 1 {
		t.Errorf("after delete: hit = %v, Len() = %d", ok, c.Len())
	}
}

func TestEvictionCallback(t *testing.T) {
	type evicted struct {
		key   string
		value int
	}
	var got []evicted
	c := newCache(t, 2, func(k string, v int) { got = append(got, evicted{k, v}) })
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Delete("b") // explicit removal does not trigger the callback
	c.Put("d", 4)
	c.Put("e", 5)

	want := []evicted{{"a", 1}, {"c", 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("evicted = %v, want %v", got, want)
	}
}
