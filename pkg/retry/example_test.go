package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/retry"
)

func ExampleDo() {
	calls := 0
	err := retry.Do(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}, retry.WithSleep(func(context.Context, time.Duration) error { return nil }))

	fmt.Println(calls, err)
	// Output: 3 <nil>
}
