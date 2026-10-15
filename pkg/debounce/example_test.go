package debounce_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/debounce"
)

func ExampleDebouncer() {
	d := debounce.New(time.Hour, func() { fmt.Println("saved") })

	// A burst of calls (each one would restart the wait)...
	d.Call()
	d.Call()
	d.Call()

	// ...runs fn only once. Flush forces it now instead of waiting an hour.
	fmt.Println(d.Flush())
	// Output:
	// saved
	// true
}
