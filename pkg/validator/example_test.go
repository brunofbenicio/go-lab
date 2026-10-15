package validator_test

import (
	"errors"
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/validator"
)

func ExampleValidate() {
	type SignUp struct {
		Email string `validate:"required,email"`
		Age   int    `validate:"min=18"`
	}

	err := validator.Validate(SignUp{Email: "nope", Age: 15})

	var errs validator.Errors
	if errors.As(err, &errs) {
		for _, fe := range errs {
			fmt.Println(fe.Field, "violates", fe.Rule)
		}
	}
	// Output:
	// Email violates email
	// Age violates min
}
