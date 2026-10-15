package workerpool

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPoolRunsEveryTaskAndBoundsConcurrency(t *testing.T) {
	const workers, tasks = 3, 60
	p := New(workers, 8)
	var done, current, peak atomic.Int32
	for i := 0; i < tasks; i++ {
		err := p.Submit(context.Background(), func() {
			n := current.Add(1)
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			runtime.Gosched()
			current.Add(-1)
			done.Add(1)
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	p.Close()

	if done.Load() != tasks {
		t.Errorf("ran %d tasks, want %d", done.Load(), tasks)
	}
	if peak.Load() > workers {
		t.Errorf("peak concurrency %d exceeds %d workers", peak.Load(), workers)
	}
}

func TestPoolSubmitAfterCloseAndDoubleClose(t *testing.T) {
	p := New(1, 0)
	p.Close()
	p.Close() // must not panic
	if err := p.Submit(context.Background(), func() {}); !errors.Is(err, ErrClosed) {
		t.Errorf("Submit after Close = %v, want ErrClosed", err)
	}
}

func TestPoolSubmitHonorsContextWhenQueueIsFull(t *testing.T) {
	p := New(1, 1)
	started, release := make(chan struct{}), make(chan struct{})
	_ = p.Submit(context.Background(), func() { close(started); <-release })
	<-started                                     // the only worker is now busy
	_ = p.Submit(context.Background(), func() {}) // fills the queue

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Submit(ctx, func() {}); !errors.Is(err, context.Canceled) {
		t.Errorf("Submit with a cancelled context = %v, want context.Canceled", err)
	}
	close(release)
	p.Close()
}

func TestMap(t *testing.T) {
	boom := errors.New("boom")
	square := func(_ context.Context, n int) (int, error) { return n * n, nil }
	failOnThree := func(_ context.Context, n int) (int, error) {
		if n == 3 {
			return 0, boom
		}
		return n, nil
	}
	tests := []struct {
		name    string
		workers int
		in      []int
		fn      func(context.Context, int) (int, error)
		want    []int // checked only when wantErr is nil
		wantErr error
	}{
		{"preserves order", 4, []int{1, 2, 3, 4, 5, 6}, square, []int{1, 4, 9, 16, 25, 36}, nil},
		{"single worker", 1, []int{3, 2, 1}, square, []int{9, 4, 1}, nil},
		{"workers below one", 0, []int{2}, square, []int{4}, nil},
		{"empty input", 2, nil, square, []int{}, nil},
		{"first error is returned", 2, []int{1, 2, 3, 4, 5}, failOnThree, nil, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Map(context.Background(), tt.workers, tt.in, tt.fn)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapCancelsContextOnError(t *testing.T) {
	boom := errors.New("boom")
	var sawCancel atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	_, err := Map(context.Background(), 2, []int{0, 1}, func(ctx context.Context, n int) (int, error) {
		if n == 0 {
			wg.Wait() // fails only after the other worker is waiting on ctx
			return 0, boom
		}
		wg.Done()
		<-ctx.Done()
		sawCancel.Store(true)
		return 0, ctx.Err()
	})
	if !errors.Is(err, boom) || !sawCancel.Load() {
		t.Errorf("err = %v, sawCancel = %v; want boom and a cancelled context", err, sawCancel.Load())
	}
}

func TestMapParentContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Map(ctx, 2, []int{1, 2, 3}, func(ctx context.Context, n int) (int, error) { return n, nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
