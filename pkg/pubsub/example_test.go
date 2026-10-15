package pubsub_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/pubsub"
)

func ExampleBroker() {
	b := pubsub.New[string]()
	events, cancel := b.Subscribe("orders", 8)
	defer cancel()

	delivered := b.Publish("orders", "order #1 created")
	fmt.Println(delivered, <-events)
	// Output: 1 order #1 created
}
