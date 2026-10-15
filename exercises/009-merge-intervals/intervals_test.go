package intervals

import (
	"errors"
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		name string
		in   []Interval
		want []Interval
	}{
		{"empty", nil, nil},
		{"single", []Interval{{1, 3}}, []Interval{{1, 3}}},
		{
			"classic",
			[]Interval{{1, 3}, {2, 6}, {8, 10}, {15, 18}},
			[]Interval{{1, 6}, {8, 10}, {15, 18}},
		},
		{"touching intervals merge", []Interval{{1, 4}, {4, 5}}, []Interval{{1, 5}}},
		{"unsorted input", []Interval{{8, 10}, {1, 3}, {2, 6}}, []Interval{{1, 6}, {8, 10}}},
		{"contained interval", []Interval{{1, 10}, {2, 3}, {4, 5}}, []Interval{{1, 10}}},
		{"identical intervals", []Interval{{2, 2}, {2, 2}}, []Interval{{2, 2}}},
		{"negative values", []Interval{{-5, -1}, {-3, 2}}, []Interval{{-5, 2}}},
		{"adjacent integers stay apart", []Interval{{1, 2}, {3, 4}}, []Interval{{1, 2}, {3, 4}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Merge(tt.in)
			if err != nil {
				t.Fatalf("Merge(%v) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Merge(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestMergeInvalid(t *testing.T) {
	_, err := Merge([]Interval{{1, 2}, {5, 3}})
	if !errors.Is(err, ErrInvalidInterval) {
		t.Errorf("Merge() error = %v, want ErrInvalidInterval", err)
	}
}

func TestMergeDoesNotModifyInput(t *testing.T) {
	in := []Interval{{8, 10}, {1, 3}, {2, 6}}
	snapshot := append([]Interval(nil), in...)
	if _, err := Merge(in); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, snapshot) {
		t.Errorf("input changed: got %v, want %v", in, snapshot)
	}
}

func TestLength(t *testing.T) {
	tests := []struct {
		iv   Interval
		want int
	}{
		{Interval{1, 1}, 1},
		{Interval{1, 5}, 5},
		{Interval{-2, 2}, 5},
	}
	for _, tt := range tests {
		if got := tt.iv.Length(); got != tt.want {
			t.Errorf("%v.Length() = %d, want %d", tt.iv, got, tt.want)
		}
	}
}
