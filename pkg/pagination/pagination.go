// Package pagination computes offset-based pagination: SQL offset and limit,
// total pages, next/previous flags, the displayed item range and a window of
// page numbers for a pager widget. It also slices in-memory data and reads the
// page parameters from a URL query.
package pagination

import (
	"net/url"
	"strconv"
)

// Defaults applied by New.
const (
	DefaultSize = 20
	MaxSize     = 100
)

// Page describes one page of a collection with Total items.
type Page struct {
	Number int // 1-based
	Size   int
	Total  int
}

// New builds a Page, normalizing bad input: Number < 1 becomes 1, Size < 1
// becomes DefaultSize, Size > MaxSize becomes MaxSize and Total < 0 becomes 0.
func New(number, size, total int) Page {
	if number < 1 {
		number = 1
	}
	if size < 1 {
		size = DefaultSize
	}
	return Page{Number: number, Size: min(size, MaxSize), Total: max(total, 0)}
}

// FromQuery reads the "page" and "per_page" parameters from q. Missing or
// invalid values fall back to the defaults of New.
func FromQuery(q url.Values, total int) Page {
	number, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("per_page"))
	return New(number, size, total)
}

// Offset is the number of items to skip (SQL OFFSET).
func (p Page) Offset() int { return (p.Number - 1) * p.Size }

// Limit is the maximum number of items on the page (SQL LIMIT).
func (p Page) Limit() int { return p.Size }

// TotalPages is the number of pages needed for Total items; 0 when empty.
func (p Page) TotalPages() int { return (p.Total + p.Size - 1) / p.Size }

// HasPrev reports whether an earlier page exists.
func (p Page) HasPrev() bool { return p.Number > 1 }

// HasNext reports whether a later page exists.
func (p Page) HasNext() bool { return p.Number < p.TotalPages() }

// Range returns the 1-based position of the first and last items on the page,
// as in "showing 21-40 of 95". Both are 0 if the page is past the end.
func (p Page) Range() (from, to int) {
	if p.Offset() >= p.Total {
		return 0, 0
	}
	return p.Offset() + 1, min(p.Offset()+p.Size, p.Total)
}

// Window returns up to n page numbers around the current page, for a pager
// such as "3 4 [5] 6 7". The window never leaves [1, TotalPages].
func (p Page) Window(n int) []int {
	pages := p.TotalPages()
	if n < 1 || pages == 0 {
		return nil
	}
	n = min(n, pages)
	start := max(1, p.Number-n/2)
	end := start + n - 1
	if end > pages {
		end = pages
		start = end - n + 1
	}
	out := make([]int, 0, n)
	for i := start; i <= end; i++ {
		out = append(out, i)
	}
	return out
}

// Slice returns the part of items that belongs to page p, or nil if the page
// is past the end. The result shares memory with items.
func Slice[T any](items []T, p Page) []T {
	if p.Offset() >= len(items) {
		return nil
	}
	return items[p.Offset():min(p.Offset()+p.Size, len(items))]
}
