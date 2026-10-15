package transport

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
)

// roundTripFunc turns a function into an http.RoundTripper, the same trick as
// http.HandlerFunc. With it, a test can fake the whole network in a few lines.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// trackedBody records whether it was closed (a leak check for retried responses).
type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

// step is one scripted outcome: either a status code or a network error.
type step struct {
	status int
	err    error
}

// scripted returns a fake network that plays steps in order and records the
// requests and response bodies it handed out.
type scripted struct {
	steps    []step
	requests []*http.Request
	bodies   []*trackedBody
	sent     [][]byte // request bodies as received
}

func (s *scripted) RoundTrip(r *http.Request) (*http.Response, error) {
	i := len(s.requests)
	s.requests = append(s.requests, r)
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		s.sent = append(s.sent, b)
	}
	if i >= len(s.steps) {
		return nil, errors.New("scripted: more requests than steps")
	}
	if st := s.steps[i]; st.err != nil {
		return nil, st.err
	}
	body := &trackedBody{Reader: strings.NewReader("body")}
	s.bodies = append(s.bodies, body)
	return &http.Response{StatusCode: s.steps[i].status, Header: http.Header{}, Body: body, Request: r}, nil
}

// delays returns a Sleep replacement that records the requested waits.
func delays(into *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		*into = append(*into, d)
		return nil
	}
}

func get(t *testing.T, rt http.RoundTripper) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/data", nil)
	if err != nil {
		t.Fatal(err)
	}
	return (&http.Client{Transport: rt}).Do(req) // no network involved
}

func TestRetryUntilSuccess(t *testing.T) {
	fake := &scripted{steps: []step{{status: 503}, {status: 502}, {status: 200}}}
	var waited []time.Duration
	rt := RetryTransport{Base: fake, MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, Sleep: delays(&waited)}

	resp, err := get(t, rt)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 || len(fake.requests) != 3 {
		t.Errorf("status = %d after %d requests, want 200 after 3", resp.StatusCode, len(fake.requests))
	}
	// Exponential backoff, verified instantly because nothing really sleeps.
	if want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}; !slices.Equal(waited, want) {
		t.Errorf("delays = %v, want %v", waited, want)
	}
	// The two discarded responses must have been closed to free the connections.
	if !fake.bodies[0].closed || !fake.bodies[1].closed {
		t.Error("discarded response bodies were not closed")
	}
}

func TestRetryGivesUpAndReturnsLastResponse(t *testing.T) {
	fake := &scripted{steps: []step{{status: 503}, {status: 503}, {status: 503}}}
	rt := RetryTransport{Base: fake, MaxAttempts: 3, Sleep: delays(new([]time.Duration))}

	resp, err := get(t, rt)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 || len(fake.requests) != 3 {
		t.Errorf("status = %d after %d requests, want 503 after 3", resp.StatusCode, len(fake.requests))
	}
}

func TestRetryOnNetworkError(t *testing.T) {
	fake := &scripted{steps: []step{{err: errors.New("connection reset")}, {status: 200}}}
	rt := RetryTransport{Base: fake, MaxAttempts: 2, Sleep: delays(new([]time.Duration))}

	resp, err := get(t, rt)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if len(fake.requests) != 2 {
		t.Errorf("requests = %d, want 2", len(fake.requests))
	}
}

func TestNoRetryForClientErrorsOrCancellation(t *testing.T) {
	t.Run("404", func(t *testing.T) {
		fake := &scripted{steps: []step{{status: 404}, {status: 200}}}
		resp, err := get(t, RetryTransport{Base: fake, MaxAttempts: 3})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if len(fake.requests) != 1 {
			t.Errorf("requests = %d, want 1", len(fake.requests))
		}
	})
	t.Run("context cancelled", func(t *testing.T) {
		fake := &scripted{steps: []step{{err: context.Canceled}, {status: 200}}}
		if _, err := get(t, RetryTransport{Base: fake, MaxAttempts: 3}); !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
		if len(fake.requests) != 1 {
			t.Errorf("requests = %d, want 1", len(fake.requests))
		}
	})
}

func TestRetryStopsWhenContextEndsDuringBackoff(t *testing.T) {
	fake := &scripted{steps: []step{{status: 503}, {status: 200}}}
	rt := RetryTransport{
		Base: fake, MaxAttempts: 3,
		Sleep: func(context.Context, time.Duration) error { return context.Canceled },
	}
	if _, err := get(t, rt); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRetryReplaysRequestBody(t *testing.T) {
	fake := &scripted{steps: []step{{status: 503}, {status: 200}}}
	rt := RetryTransport{Base: fake, MaxAttempts: 2, Sleep: delays(new([]time.Duration))}

	// bytes.Reader makes http.NewRequest set GetBody, so the body is replayable.
	req, err := http.NewRequestWithContext(t.Context(), "POST", "http://example.com/items", bytes.NewReader([]byte("payload")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: rt}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(fake.sent) != 2 || string(fake.sent[0]) != "payload" || string(fake.sent[1]) != "payload" {
		t.Errorf("bodies seen by the server = %q, want payload twice", fake.sent)
	}
}

func TestNoRetryWhenBodyCannotBeReplayed(t *testing.T) {
	fake := &scripted{steps: []step{{status: 503}, {status: 200}}}
	rt := RetryTransport{Base: fake, MaxAttempts: 3}

	// An arbitrary io.Reader leaves GetBody nil: the body is consumed once.
	body := io.NopCloser(strings.NewReader("payload"))
	req, err := http.NewRequestWithContext(t.Context(), "POST", "http://example.com/items", body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: rt}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 503 || len(fake.requests) != 1 {
		t.Errorf("status = %d after %d requests, want 503 after 1", resp.StatusCode, len(fake.requests))
	}
}

func TestBearerTransport(t *testing.T) {
	var seen *http.Request
	fake := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r
		return &http.Response{StatusCode: 200, Body: http.NoBody, Request: r}, nil
	})

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "http://example.com/x", nil)
	resp, err := BearerTransport{Token: "abc", Base: fake}.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := seen.Header.Get("Authorization"); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer abc")
	}
	// The RoundTripper contract: the caller's request must stay untouched.
	if req.Header.Get("Authorization") != "" {
		t.Error("the original request was modified")
	}
}
