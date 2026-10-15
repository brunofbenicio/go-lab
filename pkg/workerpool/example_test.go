package workerpool_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/brunofbenicio/go-lab/pkg/workerpool"
)

func ExampleMap() {
	words := []string{"go", "lab", "pool"}
	upper, err := workerpool.Map(context.Background(), 2, words,
		func(_ context.Context, s string) (string, error) { return strings.ToUpper(s), nil })
	fmt.Println(upper, err)
	// Output: [GO LAB POOL] <nil>
}
