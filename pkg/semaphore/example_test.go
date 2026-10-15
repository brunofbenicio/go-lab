package semaphore_test

import (
	"context"
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/semaphore"
)

func ExampleSemaphore() {
	s := semaphore.New(2)
	fmt.Println(s.TryAcquire(), s.TryAcquire(), s.TryAcquire())

	s.Release()
	err := s.Do(context.Background(), func() error {
		fmt.Println("in use:", s.InUse())
		return nil
	})
	fmt.Println(err)
	// Output:
	// true true false
	// in use: 2
	// <nil>
}
