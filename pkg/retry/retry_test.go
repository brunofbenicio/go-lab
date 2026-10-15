package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/backoff"
)

var errBoom = errors.New("boom")

// recorder implements WithSleep without actually waiting.
type recorder struct{ waits []time.Duration }

func (r *recorder) sleep(_ context.Context, d time.Duration) error {
	r.waits = append(r.waits, d)
	return nil
}

func TestDo(t *testing.T) {
	tests := []struct {
		name      string
		failures  int  // how many times fn fails before succeeding
		permanent bool // whether the failures are wrapped with Permanent
		opts      []Option
		wantCalls int
		wantErr   error
		wantWaits []time.Duration
	}{
		{name: "success at first call", wantCalls: 1},
		{name: "success at third call", failures: 2, wantCalls: 3,
			wantWaits: []time.Duration{time.Second, 2 * time.Second}},
		{name: "exhausted", failures: 10, opts: []Option{Attempts(2)}, wantCalls: 2,
			wantErr: ErrExhausted, wantWaits: []time.Duration{time.Second}},
		{name: "retryIf stops early", failures: 10, wantCalls: 1, wantErr: errBoom,
			opts: []Option{RetryIf(func(error) bool { return false })}},
		{name: "permanent stops early", failures: 10, permanent: true, wantCalls: 1, wantErr: errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{}
			calls := 0
			opts := append([]Option{
				WithBackoff(backoff.Linear(time.Second, 0)), WithSleep(rec.sleep),
			}, tt.opts...)
			err := Do(context.Background(), func(context.Context) error {
				calls++
				if calls > tt.failures {
					return nil
				}
				if tt.permanent {
					return Permanent(errBoom)
				}
				return errBoom
			}, opts...)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == ErrExhausted && !errors.Is(err, errBoom) {
				t.Errorf("exhausted error should wrap the last error: %v", err)
			}
			if tt.permanent && errors.Is(err, ErrExhausted) {
				t.Errorf("permanent error must not become ErrExhausted: %v", err)
			}
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if len(rec.waits) != len(tt.wantWaits) {
				t.Fatalf("waits = %v, want %v", rec.waits, tt.wantWaits)
			}
			for i, w := range tt.wantWaits {
				if rec.waits[i] != w {
					t.Errorf("wait[%d] = %v, want %v", i, rec.waits[i], w)
				}
			}
		})
	}
}

func TestDoValueAndOnRetry(t *testing.T) {
	var seen []int
	calls := 0
	got, err := DoValue(context.Background(), func(context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", errBoom
		}
		return "ok", nil
	}, WithSleep((&recorder{}).sleep), OnRetry(func(attempt int, err error, _ time.Duration) {
		if !errors.Is(err, errBoom) {
			t.Errorf("OnRetry err = %v", err)
		}
		seen = append(seen, attempt)
	}))
	if err != nil || got != "ok" {
		t.Fatalf("got (%q, %v), want (\"ok\", nil)", got, err)
	}
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Errorf("OnRetry attempts = %v, want [1 2]", seen)
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := Do(ctx, func(context.Context) error { calls++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Errorf("err = %v, calls = %d; want context.Canceled and 0 calls", err, calls)
	}

	// Cancelling during the wait keeps fn's last error.
	ctx, cancel = context.WithCancel(context.Background())
	err = Do(ctx, func(context.Context) error { return errBoom },
		WithSleep(func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		}))
	if !errors.Is(err, context.Canceled) || !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want Canceled + errBoom", err)
	}
}

func TestSleep(t *testing.T) {
	if err := sleep(context.Background(), time.Millisecond); err != nil {
		t.Errorf("sleep = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled sleep = %v, want context.Canceled", err)
	}
}

func TestPermanentNil(t *testing.T) {
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) should be nil")
	}
}
