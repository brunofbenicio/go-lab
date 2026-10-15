package pagination_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/pagination"
)

func Example() {
	p := pagination.New(3, 20, 95)
	from, to := p.Range()

	fmt.Printf("OFFSET %d LIMIT %d\n", p.Offset(), p.Limit())
	fmt.Printf("showing %d-%d of %d (page %d of %d)\n", from, to, p.Total, p.Number, p.TotalPages())
	fmt.Println(p.HasPrev(), p.HasNext(), p.Window(5))
	// Output:
	// OFFSET 40 LIMIT 20
	// showing 41-60 of 95 (page 3 of 5)
	// true true [1 2 3 4 5]
}
