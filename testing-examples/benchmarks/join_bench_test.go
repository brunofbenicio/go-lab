package idlist

import (
	"fmt"
	"testing"
)

// BenchmarkJoin compares the implementations for several input sizes using
// sub-benchmarks. Names follow "BenchmarkJoin/<impl>/n=<size>", so they can be
// filtered:
//
//	go test -bench 'Join/append' -benchmem
func BenchmarkJoin(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		ids := makeIDs(n)
		for _, impl := range implementations {
			b.Run(fmt.Sprintf("%s/n=%d", impl.name, n), func(b *testing.B) {
				b.ReportAllocs()                          // add B/op and allocs/op columns, even without -benchmem
				b.SetBytes(int64(len(impl.fn(ids, ",")))) // adds an MB/s column

				for i := 0; i < b.N; i++ {
					sink = impl.fn(ids, ",")
				}
			})
		}
	}
}

// BenchmarkJoinLoop uses b.Loop (Go 1.24+). It runs the setup before the loop
// exactly once, excludes it from the timing without b.ResetTimer, and keeps
// the compiler from optimizing the call away, so no global sink is needed.
func BenchmarkJoinLoop(b *testing.B) {
	ids := makeIDs(100)
	b.ReportAllocs()
	for b.Loop() {
		Join(ids, ",")
	}
}

// BenchmarkJoinParallel measures throughput when many goroutines call Join at
// once. Each goroutine gets its own pb; the shared ids slice is read-only.
func BenchmarkJoinParallel(b *testing.B) {
	ids := makeIDs(100)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var local string
		for pb.Next() {
			local = Join(ids, ",")
		}
		_ = local
	})
}
