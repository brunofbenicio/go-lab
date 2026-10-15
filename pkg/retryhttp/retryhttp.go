// Package retryhttp provides an http.RoundTripper that transparently retries
// failed requests with backoff. It retries network errors and the status
// codes 429, 502, 503 and 504, honors a numeric Retry-After header, and only
// retries requests that are safe to repeat: idempotent methods (or a request
// carrying an Idempotency-Key header) whose body can be replayed.
package retryhttp

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/backoff"
)

// Transport is a retrying http.RoundTripper.
type Transport struct {
	base          http.RoundTripper
	attempts      int
	backoff       backoff.Strategy
	sleep         func(context.Context, time.Duration) error
	maxRetryAfter time.Duration
}

// Option customizes a Transport.
type Option func(*Transport)

// Attempts sets the maximum number of tries per request, the first one
// included (default 3). Values < 1 are ignored.
func Attempts(n int) Option {
	return func(t *Transport) {
		if n > 0 {
			t.attempts = n
		}
	}
}

// WithBackoff sets the wait between tries (default: exponential from 100ms
// with equal jitter, capped at 5s).
func WithBackoff(s backoff.Strategy) Option { return func(t *Transport) { t.backoff = s } }

// WithSleep replaces the wait function, which lets tests run instantly. It
// must return the context's error if the context ends.
func WithSleep(fn func(context.Context, time.Duration) error) Option {
	return func(t *Transport) { t.sleep = fn }
}

// WithMaxRetryAfter caps how long a Retry-After header can make the transport
// wait (default 30s), so a misbehaving server cannot stall callers.
func WithMaxRetryAfter(d time.Duration) Option { return func(t *Transport) { t.maxRetryAfter = d } }

// New wraps base, or http.DefaultTransport if base is nil.
func New(base http.RoundTripper, opts ...Option) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	t := &Transport{
		base:          base,
		attempts:      3,
		backoff:       backoff.EqualJitter(backoff.Exponential(100*time.Millisecond, 2, 5*time.Second), nil),
		sleep:         sleep,
		maxRetryAfter: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// RoundTrip implements http.RoundTripper. When retries run out, the last
// response or error is returned as is.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for attempt := 1; ; attempt++ {
		r := req
		if attempt > 1 {
			r = req.Clone(ctx)
			if req.Body != nil && req.Body != http.NoBody {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				r.Body = body
			}
		}
		resp, err := t.base.RoundTrip(r)
		if attempt >= t.attempts || ctx.Err() != nil || !shouldRetry(req, resp, err) {
			return resp, err
		}

		wait := t.backoff(attempt - 1)
		if resp != nil {
			if d, ok := retryAfter(resp); ok {
				wait = min(d, t.maxRetryAfter)
			}
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)) // let the connection be reused
			resp.Body.Close()
		}
		if err := t.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

func shouldRetry(req *http.Request, resp *http.Response, err error) bool {
	if !safeToRepeat(req) {
		return false
	}
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// safeToRepeat reports whether the request may be sent again: its body must be
// replayable and its method idempotent (or marked with an Idempotency-Key).
func safeToRepeat(req *http.Request) bool {
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return false
	}
	switch req.Method {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, http.MethodPut, http.MethodDelete:
		return true
	}
	return req.Header.Get("Idempotency-Key") != ""
}

// retryAfter reads a Retry-After header given in seconds. The HTTP-date form
// is not supported.
func retryAfter(resp *http.Response) (time.Duration, bool) {
	secs, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || secs < 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
