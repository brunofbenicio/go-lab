package ringbuffer_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/ringbuffer"
)

func ExampleBuffer() {
	b := ringbuffer.New[int](3)
	for i := 1; i <= 5; i++ {
		b.Push(i)
	}
	fmt.Println(b.Items(), b.Last(2))
	// Output: [3 4 5] [4 5]
}
