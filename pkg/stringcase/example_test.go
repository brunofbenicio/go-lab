package stringcase_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/stringcase"
)

func Example() {
	fmt.Println(stringcase.Snake("HTTPServerConfig"))
	fmt.Println(stringcase.Kebab("userAccountID"))
	fmt.Println(stringcase.Camel("order_line_item"))
	fmt.Println(stringcase.Pascal("order line item"))
	fmt.Println(stringcase.ScreamingSnake("maxRetryCount"))
	// Output:
	// http_server_config
	// user-account-id
	// orderLineItem
	// OrderLineItem
	// MAX_RETRY_COUNT
}
