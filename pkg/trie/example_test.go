package trie_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/trie"
)

func ExampleTrie() {
	t := trie.New[int]()
	for i, word := range []string{"go", "gopher", "golang", "rust"} {
		t.Put(word, i)
	}
	fmt.Println(t.KeysWithPrefix("go"))
	fmt.Println(t.HasPrefix("py"))
	// Output:
	// [go golang gopher]
	// false
}
