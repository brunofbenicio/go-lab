// Package twosum solves the classic "two sum" problem with a hash map.
package twosum

import "slices"

// TwoSum returns the indices i < j of two elements of nums that add up to
// target. The boolean is false when no such pair exists. It runs in O(n) time
// by remembering the values already seen.
func TwoSum(nums []int, target int) (i, j int, ok bool) {
	seen := make(map[int]int, len(nums)) // value -> index of its first occurrence
	for idx, n := range nums {
		if first, found := seen[target-n]; found {
			return first, idx, true
		}
		if _, dup := seen[n]; !dup {
			seen[n] = idx
		}
	}
	return 0, 0, false
}

// Pairs returns every distinct pair of values {a, b} with a <= b that add up
// to target, sorted by a. A value is used twice only if it occurs at least
// twice in nums.
func Pairs(nums []int, target int) [][2]int {
	counts := make(map[int]int, len(nums))
	for _, n := range nums {
		counts[n]++
	}
	values := make([]int, 0, len(counts))
	for v := range counts {
		values = append(values, v)
	}
	slices.Sort(values)

	var pairs [][2]int
	for _, a := range values {
		b := target - a
		switch {
		case a < b && counts[b] > 0:
			pairs = append(pairs, [2]int{a, b})
		case a == b && counts[a] >= 2:
			pairs = append(pairs, [2]int{a, b})
		}
	}
	return pairs
}
