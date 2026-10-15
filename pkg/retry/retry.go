// Package retry runs an operation repeatedly until it succeeds, attempts run
// out or the context is cancelled, waiting between attempts according to a
// backoff strategy.
package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/backoff"
)

// ErrExhausted is returned (wrapping the last error) when every attempt
// failed.
var ErrExhausted = errors.New("retry: attempts exhausted")

type config struct {
	attempts int
	backoff  backoff.Strategy
	retryIf  func(error) bool
	onRetry  func(attempt int, err error, wait time.Duration)
	sleep    func(context.Context, time.Duration) error
}

// Option customizes the behavior of Do and DoValue.
type Option func(*config)

// Attempts sets the maximum number of executions (default 3). Values < 1 are
// ignored.
func Attempts(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.attempts = n
		}
	}
}

// WithBackoff sets the wait strategy between attempts. The default is
// exponential backoff (100ms, factor 2, 5s ceiling) with equal jitter.
func WithBackoff(s backoff.Strategy) Option { return func(c *config) { c.backoff = s } }

// RetryIf decides whether an error deserves another attempt. By default,
// every error does.
func RetryIf(fn func(error) bool) Option { return func(c *config) { c.retryIf = fn } }

// OnRetry registers a callback invoked before each wait. attempt is the
// number of the attempt that just failed (1-based).
func OnRetry(fn func(attempt int, err error, wait time.Duration)) Option {
	return func(c *config) { c.onRetry = fn }
}

// WithSleep replaces the wait function, which allows testing without a real
// clock. The function must return the context's error if it is cancelled.
func WithSleep(fn func(context.Context, time.Duration) error) Option {
	return func(c *config) { c.sleep = fn }
}

type permanent struct{ err error }

func (p *permanent) Error() string { return p.err.Error() }
func (p *permanent) Unwrap() error { return p.err }

// Permanent marks err as final: Do stops retrying and returns err without
// the marker. Permanent(nil) returns nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanent{err}
}

// Do runs fn until it returns nil. See DoValue for the variant with a result.
func Do(ctx context.Context, fn func(context.Context) error, opts ...Option) error {
	_, err := DoValue(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	}, opts...)
	return err
}

// DoValue runs fn until it returns a nil error and returns the value it
// produced. If attempts run out, the returned error satisfies errors.Is with
// both ErrExhausted and fn's last error. If the context ends, the context's
// error is returned together with the last error from fn, if any.
func DoValue[T any](ctx context.Context, fn func(context.Context) (T, error), opts ...Option) (T, error) {
	cfg := config{
		attempts: 3,
		backoff:  backoff.EqualJitter(backoff.Exponential(100*time.Millisecond, 2, 5*time.Second), nil),
		retryIf:  func(error) bool { return true },
		sleep:    sleep,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	var zero T
	var err error
	for attempt := 1; attempt <= cfg.attempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return zero, errors.Join(cerr, err)
		}
		var v T
		if v, err = fn(ctx); err == nil {
			return v, nil
		}
		var p *permanent
		if errors.As(err, &p) {
			return zero, p.err
		}
		if !cfg.retryIf(err) {
			return zero, err
		}
		if attempt == cfg.attempts {
			break
		}
		wait := cfg.backoff(attempt - 1)
		if cfg.onRetry != nil {
			cfg.onRetry(attempt, err, wait)
		}
		if serr := cfg.sleep(ctx, wait); serr != nil {
			return zero, errors.Join(serr, err)
		}
	}
	return zero, fmt.Errorf("%w after %d attempts: %w", ErrExhausted, cfg.attempts, err)
}

// sleep waits for d or until the context ends.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
