package priorityqueue_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/priorityqueue"
)

func ExampleQueue() {
	type job struct {
		name     string
		priority int
	}
	q := priorityqueue.New(func(a, b job) bool { return a.priority > b.priority })
	q.Push(job{"backup", 1})
	q.Push(job{"deploy", 9})
	q.Push(job{"email", 5})

	for q.Len() > 0 {
		j, _ := q.Pop()
		fmt.Println(j.name)
	}
	// Output:
	// deploy
	// email
	// backup
}
