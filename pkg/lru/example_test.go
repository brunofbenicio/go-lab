package lru_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/lru"
)

func ExampleCache() {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // "a" becomes the most recently used
	c.Put("c", 3) // evicts "b"

	_, ok := c.Get("b")
	fmt.Println(ok, c.Keys())
	// Output: false [c a]
}
