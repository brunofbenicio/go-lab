package consistenthash_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/consistenthash"
)

func ExampleRing() {
	r := consistenthash.New(50)
	r.Add("cache-1", "cache-2", "cache-3")

	first, _ := r.Get("user:42")
	again, _ := r.Get("user:42")
	fmt.Println(first == again) // the same key always maps to the same node

	r.Remove(first)
	moved, _ := r.Get("user:42")
	fmt.Println(moved != first)
	// Output:
	// true
	// true
}
