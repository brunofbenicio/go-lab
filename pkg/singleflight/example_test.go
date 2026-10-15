package singleflight_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/singleflight"
)

func ExampleGroup() {
	var g singleflight.Group[string, string]

	v, shared, err := g.Do("user:42", func() (string, error) {
		return "loaded from the database", nil
	})
	fmt.Println(v, shared, err)
	// Output: loaded from the database false <nil>
}
