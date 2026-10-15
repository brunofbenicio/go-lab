package ttlcache_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/ttlcache"
)

func ExampleCache() {
	now := time.Unix(0, 0)
	c := ttlcache.New[string, string](time.Minute, ttlcache.WithClock(func() time.Time { return now }))

	c.Set("session", "abc123")
	v, ok := c.Get("session")
	fmt.Println(v, ok)

	now = now.Add(time.Minute)
	_, ok = c.Get("session")
	fmt.Println(ok)
	// Output:
	// abc123 true
	// false
}
