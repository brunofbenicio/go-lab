package twosum

import (
	"reflect"
	"testing"
)

func TestTwoSum(t *testing.T) {
	tests := []struct {
		name       string
		nums       []int
		target     int
		wantI      int
		wantJ      int
		wantExists bool
	}{
		{"classic", []int{2, 7, 11, 15}, 9, 0, 1, true},
		{"not at the start", []int{3, 2, 4}, 6, 1, 2, true},
		{"equal values", []int{3, 3}, 6, 0, 1, true},
		{"negative numbers", []int{-1, -2, -3, -4, -5}, -8, 2, 4, true},
		{"zero target", []int{4, -4, 9}, 0, 0, 1, true},
		{"element cannot be reused", []int{3, 5}, 6, 0, 0, false},
		{"no solution", []int{1, 2, 3}, 100, 0, 0, false},
		{"empty", nil, 5, 0, 0, false},
		{"single element", []int{5}, 10, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, j, ok := TwoSum(tt.nums, tt.target)
			if ok != tt.wantExists || i != tt.wantI || j != tt.wantJ {
				t.Errorf("TwoSum(%v, %d) = (%d, %d, %v), want (%d, %d, %v)",
					tt.nums, tt.target, i, j, ok, tt.wantI, tt.wantJ, tt.wantExists)
			}
			if ok && (i == j || tt.nums[i]+tt.nums[j] != tt.target) {
				t.Errorf("TwoSum(%v, %d) returned invalid indices (%d, %d)", tt.nums, tt.target, i, j)
			}
		})
	}
}

func TestPairs(t *testing.T) {
	tests := []struct {
		name   string
		nums   []int
		target int
		want   [][2]int
	}{
		{"several pairs", []int{1, 5, 7, -1, 5, 3, 4}, 6, [][2]int{{-1, 7}, {1, 5}}},
		{"duplicates count once", []int{2, 2, 2, 4, 4}, 6, [][2]int{{2, 4}}},
		{"same value needs two copies", []int{3, 1}, 6, nil},
		{"same value with two copies", []int{3, 3, 1}, 6, [][2]int{{3, 3}}},
		{"no pairs", []int{1, 2}, 10, nil},
		{"empty", nil, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Pairs(tt.nums, tt.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Pairs(%v, %d) = %v, want %v", tt.nums, tt.target, got, tt.want)
			}
		})
	}
}
