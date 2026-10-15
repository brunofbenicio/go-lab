// Package kadane finds the contiguous subarray with the largest sum using
// Kadane's algorithm in O(n) time and O(1) extra space.
package kadane

// Result describes a contiguous subarray xs[Start:End+1].
type Result struct {
	Sum   int // sum of the elements in the subarray
	Start int // index of the first element (inclusive)
	End   int // index of the last element (inclusive)
}

// MaxSubarray returns the non-empty contiguous subarray of xs with the
// largest sum. When several subarrays tie, the one found first (leftmost end)
// wins. The boolean is false when xs is empty. If every element is negative,
// the result is the single largest element.
func MaxSubarray(xs []int) (Result, bool) {
	if len(xs) == 0 {
		return Result{}, false
	}
	best := Result{Sum: xs[0]}
	cur, curStart := xs[0], 0
	for i := 1; i < len(xs); i++ {
		// A negative running sum can only hurt: restart at the current element.
		if cur < 0 {
			cur, curStart = xs[i], i
		} else {
			cur += xs[i]
		}
		if cur > best.Sum {
			best = Result{Sum: cur, Start: curStart, End: i}
		}
	}
	return best, true
}
