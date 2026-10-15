package lru

import (
	"slices"
	"sync"
	"testing"
)

func TestEvictionOrder(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		ops      func(c *Cache[string, int])
		wantKeys []string // most to least recently used
	}{
		{"fills without eviction", 3, func(c *Cache[string, int]) {
			c.Put("a", 1)
			c.Put("b", 2)
		}, []string{"b", "a"}},
		{"evicts the oldest", 2, func(c *Cache[string, int]) {
			c.Put("a", 1)
			c.Put("b", 2)
			c.Put("c", 3)
		}, []string{"c", "b"}},
		{"Get promotes", 2, func(c *Cache[string, int]) {
			c.Put("a", 1)
			c.Put("b", 2)
			c.Get("a")
			c.Put("c", 3)
		}, []string{"c", "a"}},
		{"Put on existing key promotes and updates", 2, func(c *Cache[string, int]) {
			c.Put("a", 1)
			c.Put("b", 2)
			c.Put("a", 10)
			c.Put("c", 3)
		}, []string{"c", "a"}},
		{"Peek does not promote", 2, func(c *Cache[string, int]) {
			c.Put("a", 1)
			c.Put("b", 2)
			c.Peek("a")
			c.Put("c", 3)
		}, []string{"c", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New[string, int](tt.capacity)
			tt.ops(c)
			if got := c.Keys(); !slices.Equal(got, tt.wantKeys) {
				t.Errorf("Keys() = %v, want %v", got, tt.wantKeys)
			}
			if c.Len() != len(tt.wantKeys) {
				t.Errorf("Len() = %d, want %d", c.Len(), len(tt.wantKeys))
			}
		})
	}
}

func TestGetPutRemove(t *testing.T) {
	c := New[string, int](2)
	if _, ok := c.Get("missing"); ok {
		t.Error("Get on an empty cache should miss")
	}
	c.Put("a", 1)
	c.Put("a", 2)
	if v, ok := c.Get("a"); !ok || v != 2 {
		t.Errorf("Get(a) = %d, %v; want 2, true", v, ok)
	}
	if !c.Remove("a") || c.Remove("a") {
		t.Error("Remove should report true once, then false")
	}
	if c.Len() != 0 {
		t.Errorf("Len() = %d, want 0", c.Len())
	}
}

func TestOnEvict(t *testing.T) {
	c := New[string, int](1)
	var gotKey string
	var gotVal int
	c.OnEvict(func(k string, v int) { gotKey, gotVal = k, v })

	if c.Put("a", 1) {
		t.Error("first Put should not evict")
	}
	if !c.Put("b", 2) {
		t.Error("second Put should evict")
	}
	if gotKey != "a" || gotVal != 1 {
		t.Errorf("evicted (%q, %d), want (a, 1)", gotKey, gotVal)
	}
	gotKey = ""
	c.Remove("b")
	if gotKey != "" {
		t.Error("Remove must not call OnEvict")
	}
}

func TestNewPanicsOnInvalidCapacity(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(0) should panic")
		}
	}()
	New[int, int](0)
}

func TestConcurrentUse(t *testing.T) {
	c := New[int, int](8)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				c.Put(i%20, i)
				c.Get((i + g) % 20)
			}
		}()
	}
	wg.Wait()
	if c.Len() > 8 {
		t.Errorf("Len() = %d exceeds capacity 8", c.Len())
	}
}
