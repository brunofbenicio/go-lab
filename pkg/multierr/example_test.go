package multierr_test

import (
	"errors"
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/multierr"
)

func ExampleCombine() {
	err := multierr.Combine(errors.New("disk full"), nil, errors.New("permission denied"))
	fmt.Println(err)
	fmt.Println(len(multierr.Errors(err)))
	// Output:
	// 2 errors occurred:
	// 	* disk full
	// 	* permission denied
	// 2
}

func ExampleAppendInto() {
	save := func() (err error) {
		defer func() { multierr.AppendInto(&err, errors.New("close failed")) }()
		return errors.New("write failed")
	}
	fmt.Println(save())
	// Output:
	// 2 errors occurred:
	// 	* write failed
	// 	* close failed
}
