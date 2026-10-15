package slugify_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/slugify"
)

func ExampleMake() {
	fmt.Println(slugify.Make("Ação e Reação: um guia rápido!"))
	fmt.Println(slugify.Make("Hello, World!", slugify.WithSeparator("_")))
	fmt.Println(slugify.Make("The quick brown fox", slugify.WithMaxLength(12)))
	// Output:
	// acao-e-reacao-um-guia-rapido
	// hello_world
	// the-quick
}
