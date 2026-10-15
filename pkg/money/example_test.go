package money_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/money"
)

func Example() {
	price, _ := money.Parse("19.90", "BRL")
	total, _ := price.Mul(3)
	fmt.Println(total)

	shares, _ := total.Allocate(1, 1, 1) // split the bill without losing a cent
	fmt.Println(shares)
	// Output:
	// 59.70 BRL
	// [19.90 BRL 19.90 BRL 19.90 BRL]
}
