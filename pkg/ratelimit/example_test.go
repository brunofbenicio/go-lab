package ratelimit_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/ratelimit"
)

func ExampleLimiter() {
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }

	l := ratelimit.New(1, 2, ratelimit.WithClock(clock)) // 1 token/s, burst of 2
	fmt.Println(l.Allow(), l.Allow(), l.Allow())

	now = now.Add(time.Second)
	fmt.Println(l.Allow())
	// Output:
	// true true false
	// true
}
