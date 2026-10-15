package deque_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/deque"
)

func ExampleDeque() {
	var d deque.Deque[int]
	d.PushBack(2)
	d.PushBack(3)
	d.PushFront(1)

	first, _ := d.PopFront()
	last, _ := d.PopBack()
	fmt.Println(first, last, d.Len())
	// Output: 1 3 1
}
