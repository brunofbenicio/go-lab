package truncate_test

import (
	"fmt"

	"github.com/brunofbenicio/go-lab/pkg/truncate"
)

func Example() {
	fmt.Println(truncate.String("Ação rápida e segura", 10, "…"))
	fmt.Println(truncate.Words("the quick brown fox", 15, "…"))
	fmt.Println(truncate.Middle("quarterly-report-final.pdf", 14, "…"))
	fmt.Println(truncate.Bytes("ação", 3))
	// Output:
	// Ação rápi…
	// the quick…
	// quarter…al.pdf
	// aç
}
