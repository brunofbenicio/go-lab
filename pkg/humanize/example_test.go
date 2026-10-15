package humanize_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/humanize"
)

func Example() {
	fmt.Println(humanize.Bytes(1_500_000))
	fmt.Println(humanize.IBytes(3 << 20))
	fmt.Println(humanize.Duration(90*time.Minute + 20*time.Second))
	fmt.Println(humanize.Thousands(1234567, "."))
	// Output:
	// 1.5 MB
	// 3 MiB
	// 1h 30m 20s
	// 1.234.567
}
