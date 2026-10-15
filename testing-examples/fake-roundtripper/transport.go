// Package transport provides two http.RoundTripper middlewares: BearerTransport
// adds an Authorization header, and RetryTransport retries transient failures
// with exponential backoff.
package transport

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// BearerTransport sets "Authorization: Bearer <Token>" on every request.
type BearerTransport struct {
	Token string
	Base  http.RoundTripper // nil means http.DefaultTransport
}

// RoundTrip implements http.RoundTripper. As the interface requires, it does
// not modify the caller's request: it sends a clone.
func (t BearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.Token)
	return base(t.Base).RoundTrip(clone)
}

// RetryTransport retries a request when the network fails or the server
// answers 502, 503 or 504. Requests whose body cannot be replayed are never
// retried.
type RetryTransport struct {
	Base        http.RoundTripper // nil means http.DefaultTransport
	MaxAttempts int               // total tries, including the first; values below 1 mean 1
	BaseDelay   time.Duration     // wait before the first retry; doubles each time

	// Sleep waits for d or until ctx is done. Nil uses a real timer; tests
	// replace it so nothing actually sleeps.
	Sleep func(ctx context.Context, d time.Duration) error
}

// RoundTrip implements http.RoundTripper.
func (t RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := max(t.MaxAttempts, 1)
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	sleep := t.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	delay := t.BaseDelay
	for attempt := 1; ; attempt++ {
		r := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r.Body = body
		}

		resp, err := base(t.Base).RoundTrip(r)
		if !retryable(resp, err) || attempt >= attempts || !replayable {
			return resp, err
		}
		if resp != nil {
			resp.Body.Close() // we are discarding this response
		}
		if err := sleep(req.Context(), delay); err != nil {
			return nil, err
		}
		delay *= 2
	}
}

func retryable(resp *http.Response, err error) bool {
	if err != nil {
		return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
	}
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func base(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return http.DefaultTransport
	}
	return rt
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
