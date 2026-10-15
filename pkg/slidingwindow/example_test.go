package slidingwindow_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/slidingwindow"
)

func ExampleLimiter() {
	now := time.Unix(0, 0)
	l := slidingwindow.New(2, time.Minute, slidingwindow.WithClock(func() time.Time { return now }))

	fmt.Println(l.Allow("user-1"), l.Allow("user-1"), l.Allow("user-1"))
	fmt.Println(l.RetryAfter("user-1"))
	// Output:
	// true true false
	// 1m0s
}
