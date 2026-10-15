package todo

import (
	"context"
	"fmt"
	"testing"
)

func filledStore(b *testing.B, n int) *MemoryStore {
	b.Helper()
	s := NewMemoryStore()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		in := Input{Title: fmt.Sprintf("task number %d", i), Notes: "some notes", Done: i%3 == 0}
		if _, err := s.Create(ctx, in); err != nil {
			b.Fatal(err)
		}
	}
	return s
}

func BenchmarkMemoryStoreCreate(b *testing.B) {
	s := NewMemoryStore()
	ctx := context.Background()
	in := Input{Title: "benchmark", Notes: "notes"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Create(ctx, in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryStoreGet(b *testing.B) {
	s := filledStore(b, 1000)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Get(ctx, int64(i%1000)+1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryStoreGetParallel(b *testing.B) {
	s := filledStore(b, 1000)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var i int64
		for pb.Next() {
			i++
			if _, err := s.Get(ctx, i%1000+1); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkMemoryStoreList shows how listing scales with the collection size
// and how much filtering and paging cost on top of a plain scan.
func BenchmarkMemoryStoreList(b *testing.B) {
	done := true
	cases := []struct {
		name string
		opts ListOptions
	}{
		{"all", ListOptions{}},
		{"page-of-50", ListOptions{Limit: 50, Offset: 100}},
		{"done-only", ListOptions{Done: &done}},
		{"search", ListOptions{Query: "number 9"}},
	}

	for _, size := range []int{100, 1000, 10000} {
		s := filledStore(b, size)
		for _, c := range cases {
			b.Run(fmt.Sprintf("n=%d/%s", size, c.name), func(b *testing.B) {
				ctx := context.Background()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := s.List(ctx, c.opts); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
