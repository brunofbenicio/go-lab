package bloom_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/bloom"
)

func ExampleFilter() {
	f := bloom.New(100, 0.01)
	f.AddString("alice@example.com")

	fmt.Println(f.MayContainString("alice@example.com"))
	fmt.Println(f.MayContainString("bob@example.com"))
	// Output:
	// true
	// false
}
