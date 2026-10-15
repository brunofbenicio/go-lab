package uuid_test

import (
	"bytes"
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/uuid"
)

func ExampleNew() {
	id := uuid.New()
	fmt.Println(len(id.String()), id.Version())
	// Output: 36 4
}

func ExampleParse() {
	id, err := uuid.Parse("123E4567-E89B-42D3-A456-426614174000")
	fmt.Println(id, err)
	// Output: 123e4567-e89b-42d3-a456-426614174000 <nil>
}

func ExampleNewFromReader() {
	id, _ := uuid.NewFromReader(bytes.NewReader(make([]byte, 16)))
	fmt.Println(id)
	// Output: 00000000-0000-4000-8000-000000000000
}
