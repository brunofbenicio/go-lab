package sliceutil_test

import (
	"fmt"
	"strings"

	"github.com/brunofbenicio/go-lab/pkg/sliceutil"
)

func Example() {
	words := []string{"go", "lab", "go", "pkg", "util"}

	upper := sliceutil.Map(words, strings.ToUpper)
	long := sliceutil.Filter(words, func(s string) bool { return len(s) > 2 })
	total := sliceutil.Reduce(words, 0, func(n int, s string) int { return n + len(s) })

	fmt.Println(upper)
	fmt.Println(sliceutil.Unique(long), total)
	fmt.Println(sliceutil.Chunk(words, 2))
	// Output:
	// [GO LAB GO PKG UTIL]
	// [lab pkg util] 14
	// [[go lab] [go pkg] [util]]
}
