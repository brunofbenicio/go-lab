package backoff_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/backoff"
)

func ExampleExponential() {
	s := backoff.Exponential(100*time.Millisecond, 2, time.Second)
	for attempt := 0; attempt < 5; attempt++ {
		fmt.Println(s(attempt))
	}
	// Output:
	// 100ms
	// 200ms
	// 400ms
	// 800ms
	// 1s
}
