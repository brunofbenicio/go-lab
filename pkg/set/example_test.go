package set_test

import (
	"fmt"
	"slices"

	"github.com/brunofbenicio/go-lab/pkg/set"
)

func ExampleSet() {
	a, b := set.New(1, 2, 3), set.New(3, 4)

	union := a.Union(b).Slice()
	slices.Sort(union)
	fmt.Println(union)
	fmt.Println(a.Intersect(b).Has(3), a.Difference(b).Len())
	// Output:
	// [1 2 3 4]
	// true 2
}
