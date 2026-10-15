// Package intervals merges overlapping closed integer intervals.
package intervals

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidInterval is returned when an interval has Start > End.
var ErrInvalidInterval = errors.New("intervals: start is greater than end")

// Interval is the closed range [Start, End].
type Interval struct {
	Start, End int
}

// Merge returns the smallest sorted list of disjoint intervals that covers
// the same points as in. Intervals that overlap or touch ([1,2] and [2,3])
// are combined. The input slice is not modified.
func Merge(in []Interval) ([]Interval, error) {
	if len(in) == 0 {
		return nil, nil
	}
	sorted := slices.Clone(in)
	for _, iv := range sorted {
		if iv.Start > iv.End {
			return nil, fmt.Errorf("%w: [%d, %d]", ErrInvalidInterval, iv.Start, iv.End)
		}
	}
	slices.SortFunc(sorted, func(a, b Interval) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
	})

	merged := []Interval{sorted[0]}
	for _, iv := range sorted[1:] {
		last := &merged[len(merged)-1]
		if iv.Start <= last.End {
			last.End = max(last.End, iv.End)
			continue
		}
		merged = append(merged, iv)
	}
	return merged, nil
}

// Length returns how many integer points are covered by the closed interval.
func (iv Interval) Length() int {
	return iv.End - iv.Start + 1
}
