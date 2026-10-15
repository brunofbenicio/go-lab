package semaphore

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTryAcquireAndRelease(t *testing.T) {
	s := New(2)
	if !s.TryAcquire() || !s.TryAcquire() {
		t.Fatal("the first two TryAcquire calls should succeed")
	}
	if s.TryAcquire() {
		t.Error("third TryAcquire should fail on a full semaphore")
	}
	if s.InUse() != 2 || s.Cap() != 2 {
		t.Errorf("InUse/Cap = %d/%d, want 2/2", s.InUse(), s.Cap())
	}
	s.Release()
	if !s.TryAcquire() {
		t.Error("TryAcquire should succeed after Release")
	}
}

func TestAcquireWithContext(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		prefill int
		ctx     context.Context
		wantErr error
	}{
		{"free slot", 0, context.Background(), nil},
		{"full but context cancelled", 1, cancelled, context.Canceled},
		{"free slot but context already cancelled", 0, cancelled, context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(1)
			for i := 0; i < tt.prefill; i++ {
				s.TryAcquire()
			}
			if err := s.Acquire(tt.ctx); !errors.Is(err, tt.wantErr) {
				t.Errorf("Acquire = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAcquireBlocksUntilRelease(t *testing.T) {
	s := New(1)
	s.TryAcquire()
	acquired := make(chan struct{})
	go func() {
		_ = s.Acquire(context.Background())
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("Acquire returned while the semaphore was full")
	default:
	}
	s.Release()
	<-acquired
}

func TestReleaseWithoutAcquirePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Release on an empty semaphore should panic")
		}
	}()
	New(1).Release()
}

func TestNewPanicsOnInvalidSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(0) should panic")
		}
	}()
	New(0)
}

func TestDoLimitsConcurrency(t *testing.T) {
	const limit, goroutines = 3, 30
	s := New(limit)
	var current, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Do(context.Background(), func() error {
				n := current.Add(1)
				for {
					old := peak.Load()
					if n <= old || peak.CompareAndSwap(old, n) {
						break
					}
				}
				runtime.Gosched()
				current.Add(-1)
				return nil
			})
		}()
	}
	wg.Wait()
	if peak.Load() > limit {
		t.Errorf("peak concurrency %d exceeds the limit %d", peak.Load(), limit)
	}
	if s.InUse() != 0 {
		t.Errorf("InUse() = %d after all work finished, want 0", s.InUse())
	}
}

func TestDoReleasesOnErrorAndPanic(t *testing.T) {
	s := New(1)
	boom := errors.New("boom")
	if err := s.Do(context.Background(), func() error { return boom }); !errors.Is(err, boom) {
		t.Errorf("Do = %v, want boom", err)
	}
	func() {
		defer func() { _ = recover() }()
		_ = s.Do(context.Background(), func() error { panic("oops") })
	}()
	if s.InUse() != 0 {
		t.Errorf("InUse() = %d, want 0 after error and panic", s.InUse())
	}
}
