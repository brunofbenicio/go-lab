package idlist

import (
	"slices"
	"testing"
)

var implementations = []struct {
	name string
	fn   func([]int, string) string
}{
	{"JoinNaive", JoinNaive},
	{"JoinBuilder", JoinBuilder},
	{"Join", Join},
}

// Before comparing speed, prove that every implementation computes the same
// thing: a benchmark of a wrong function is meaningless.
func TestImplementationsAgree(t *testing.T) {
	tests := []struct {
		name string
		ids  []int
		sep  string
		want string
	}{
		{"nil", nil, ",", ""},
		{"single", []int{42}, ",", "42"},
		{"several", []int{1, 22, 333}, ",", "1,22,333"},
		{"negative and zero", []int{-5, 0, 7}, ", ", "-5, 0, 7"},
		{"empty separator", []int{1, 2, 3}, "", "123"},
		{"long numbers", []int{1234567890123, -987654321}, "|", "1234567890123|-987654321"},
	}
	for _, impl := range implementations {
		for _, tt := range tests {
			t.Run(impl.name+"/"+tt.name, func(t *testing.T) {
				if got := impl.fn(tt.ids, tt.sep); got != tt.want {
					t.Errorf("%s(%v, %q) = %q, want %q", impl.name, tt.ids, tt.sep, got, tt.want)
				}
			})
		}
	}
}

// testing.AllocsPerRun turns a performance property into a regular test: it
// fails if someone reintroduces per-item allocations. It averages many runs,
// so the value is stable (and exact for deterministic code like this).
func TestJoinAllocations(t *testing.T) {
	ids := makeIDs(500)
	allocs := testing.AllocsPerRun(50, func() { sink = Join(ids, ",") })
	if allocs > 2 { // one for the buffer, one for the final string
		t.Errorf("Join allocates %.0f times per call, want at most 2", allocs)
	}
}

func makeIDs(n int) []int {
	ids := make([]int, n)
	for i := range ids {
		ids[i] = 1000 + i*7
	}
	return ids
}

var sink string // keeps results alive so the compiler cannot discard the calls

func TestMakeIDs(t *testing.T) {
	if got := makeIDs(3); !slices.Equal(got, []int{1000, 1007, 1014}) {
		t.Errorf("makeIDs(3) = %v", got)
	}
}
