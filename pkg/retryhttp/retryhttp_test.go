package retryhttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/backoff"
)

var errNet = errors.New("connection reset")

type trackedBody struct {
	io.Reader
	closed *bool
}

func (b trackedBody) Close() error { *b.closed = true; return nil }

// fake plays scripted outcomes (an int status or an error; the last one
// repeats) and records what it saw.
type fake struct {
	outcomes []any
	header   http.Header
	calls    int
	bodies   []string // request bodies, one per call that had one
	closed   []*bool  // one flag per response, set when its body is closed
}

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		f.bodies = append(f.bodies, string(b))
	}
	out := f.outcomes[min(f.calls, len(f.outcomes)-1)]
	f.calls++
	if err, ok := out.(error); ok {
		return nil, err
	}
	closed := new(bool)
	f.closed = append(f.closed, closed)
	return &http.Response{StatusCode: out.(int), Header: f.header, Body: trackedBody{strings.NewReader("x"), closed}}, nil
}

// run sends req through a Transport over f; the returned slice holds every
// wait the transport asked for, without actually waiting.
func run(f *fake, req *http.Request, opts ...Option) (*http.Response, []time.Duration, error) {
	var waits []time.Duration
	opts = append([]Option{
		WithBackoff(backoff.Constant(time.Second)),
		WithSleep(func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }),
	}, opts...)
	resp, err := New(f, opts...).RoundTrip(req)
	return resp, waits, err
}

func newReq(t *testing.T, method string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, "http://example.com/x", body)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestRetryBehavior(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		outcomes   []any
		attempts   int // 0 keeps the default of 3
		wantCalls  int
		wantStatus int // 0 when the last network error is expected
	}{
		{"succeeds first time", "GET", []any{200}, 0, 1, 200},
		{"retries 503 until success", "GET", []any{503, 503, 200}, 0, 3, 200},
		{"gives up and returns the last response", "GET", []any{503}, 0, 3, 503},
		{"does not retry 404 or 500", "GET", []any{500}, 0, 1, 500},
		{"retries 429, 502 and 504", "GET", []any{429, 502, 504, 200}, 4, 4, 200},
		{"retries network errors", "GET", []any{errNet, 200}, 0, 2, 200},
		{"returns the last network error", "GET", []any{errNet}, 2, 2, 0},
		{"PUT is idempotent", "PUT", []any{503, 200}, 0, 2, 200},
		{"POST is not retried", "POST", []any{503, 200}, 0, 1, 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fake{outcomes: tt.outcomes}
			var opts []Option
			if tt.attempts > 0 {
				opts = append(opts, Attempts(tt.attempts))
			}
			resp, _, err := run(f, newReq(t, tt.method, nil), opts...)
			if f.calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", f.calls, tt.wantCalls)
			}
			if tt.wantStatus == 0 {
				if !errors.Is(err, errNet) {
					t.Errorf("err = %v, want errNet", err)
				}
			} else if err != nil || resp.StatusCode != tt.wantStatus {
				t.Errorf("got (%v, %v), want status %d", resp, err, tt.wantStatus)
			}
		})
	}
}

func TestWaitsAndRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		opts   []Option
		want   []time.Duration
	}{
		{"uses the backoff strategy", "", nil, []time.Duration{time.Second, time.Second}},
		{"honors Retry-After", "7", nil, []time.Duration{7 * time.Second, 7 * time.Second}},
		{"caps Retry-After", "3600", []Option{WithMaxRetryAfter(10 * time.Second)}, []time.Duration{10 * time.Second, 10 * time.Second}},
		{"ignores the unsupported date form", "Wed, 21 Oct 2026 07:28:00 GMT", nil, []time.Duration{time.Second, time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fake{outcomes: []any{503}, header: http.Header{"Retry-After": {tt.header}}}
			if _, waits, _ := run(f, newReq(t, "GET", nil), tt.opts...); !slices.Equal(waits, tt.want) {
				t.Errorf("waits = %v, want %v", waits, tt.want)
			}
		})
	}
}

func TestDiscardedResponsesAreClosed(t *testing.T) {
	f := &fake{outcomes: []any{503, 503, 200}}
	if _, _, err := run(f, newReq(t, "GET", nil)); err != nil {
		t.Fatal(err)
	}
	if !*f.closed[0] || !*f.closed[1] || *f.closed[2] {
		t.Errorf("closed flags = %v %v %v; want retried bodies closed and the final one left to the caller",
			*f.closed[0], *f.closed[1], *f.closed[2])
	}
}

func TestRequestBodyReplay(t *testing.T) {
	t.Run("replayable body with an idempotency key", func(t *testing.T) {
		f := &fake{outcomes: []any{503, 200}}
		req := newReq(t, "POST", bytes.NewReader([]byte("payload")))
		req.Header.Set("Idempotency-Key", "abc")
		if _, _, err := run(f, req); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(f.bodies, []string{"payload", "payload"}) {
			t.Errorf("bodies seen = %q, want the payload twice", f.bodies)
		}
	})

	t.Run("a body that cannot be replayed is never retried", func(t *testing.T) {
		f := &fake{outcomes: []any{503, 200}}
		req := newReq(t, "PUT", io.NopCloser(strings.NewReader("data"))) // no GetBody
		if resp, _, _ := run(f, req); f.calls != 1 || resp.StatusCode != 503 {
			t.Errorf("calls = %d; want a single call returning 503", f.calls)
		}
	})
}

func TestContextEndsDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fake{outcomes: []any{503}}
	tr := New(f, WithSleep(func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }))
	if _, err := tr.RoundTrip(newReq(t, "GET", nil).WithContext(ctx)); !errors.Is(err, context.Canceled) || f.calls != 1 {
		t.Errorf("err = %v, calls = %d; want context.Canceled after 1 call", err, f.calls)
	}
	if err := sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("default sleep with a cancelled context = %v", err)
	}
	if err := sleep(context.Background(), time.Millisecond); err != nil {
		t.Errorf("default sleep = %v, want nil", err)
	}
}
