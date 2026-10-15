// Package report renders an inventory as a fixed-width text table. Output of
// this kind is tedious to spell out inside a test, which is what golden files
// are for.
package report

import (
	"fmt"
	"io"
	"strings"
)

// Row is one line of the inventory. Prices are in cents.
type Row struct {
	Name  string
	Qty   int
	Price int64
}

// money formats cents as a decimal amount, e.g. 1999 -> "19.99".
func money(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// Render writes the table to w: a header, one line per row, and a total.
// Column widths adapt to the longest name (measured in runes).
func Render(w io.Writer, rows []Row) error {
	width := len("ITEM")
	for _, r := range rows {
		width = max(width, len([]rune(r.Name)))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-*s  %5s  %10s\n", width, "ITEM", "QTY", "TOTAL")
	b.WriteString(strings.Repeat("-", width+2+5+2+10) + "\n")

	var grand int64
	for _, r := range rows {
		total := int64(r.Qty) * r.Price
		grand += total
		fmt.Fprintf(&b, "%-*s  %5d  %10s\n", width, r.Name, r.Qty, money(total))
	}
	if len(rows) == 0 {
		b.WriteString("(no items)\n")
	}
	fmt.Fprintf(&b, "%-*s  %5s  %10s\n", width, "TOTAL", "", money(grand))

	_, err := io.WriteString(w, b.String())
	return err
}
