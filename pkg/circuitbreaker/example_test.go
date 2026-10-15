package circuitbreaker_test

import (
	"errors"
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/circuitbreaker"
)

func ExampleBreaker() {
	b := circuitbreaker.New(circuitbreaker.WithThreshold(2))
	failing := func() error { return errors.New("service down") }

	fmt.Println(b.Execute(failing))
	fmt.Println(b.Execute(failing))
	fmt.Println(b.State())
	fmt.Println(b.Execute(failing)) // rejected without calling the service
	// Output:
	// service down
	// service down
	// open
	// circuitbreaker: circuit is open
}
