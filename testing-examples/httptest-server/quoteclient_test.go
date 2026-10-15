package quoteclient

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newClient starts a real HTTP server on 127.0.0.1 (random free port) running
// handler, and returns a Client pointed at it. Closing the server is
// registered with t.Cleanup, so tests cannot forget it.
func newClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, Token: "test-token", HTTP: srv.Client()}
}

func TestQuoteSuccess(t *testing.T) {
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The handler is also where the *request* is verified. Use t.Errorf
		// here: t.Fatal must not be called from a goroutine other than the test's.
		if r.Method != http.MethodGet || r.URL.Path != "/quotes/ABC" {
			t.Errorf("request = %s %s, want GET /quotes/ABC", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"symbol":"ABC","price":12.34,"currency":"USD"}`))
	}))

	got, err := client.Quote(t.Context(), "ABC")
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if want := (Quote{Symbol: "ABC", Price: 12.34, Currency: "USD"}); got != want {
		t.Errorf("Quote = %+v, want %+v", got, want)
	}
}

func TestQuoteEscapesSymbol(t *testing.T) {
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/quotes/A%2FB%20C" {
			t.Errorf("escaped path = %q, want /quotes/A%%2FB%%20C", got)
		}
		w.Write([]byte(`{}`))
	}))
	if _, err := client.Quote(t.Context(), "A/B C"); err != nil {
		t.Fatalf("Quote: %v", err)
	}
}

func TestQuoteStatusMapping(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		check   func(t *testing.T, err error)
	}{
		{
			name:    "404 becomes ErrNotFound",
			handler: func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("error = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "429 carries Retry-After",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			check: func(t *testing.T, err error) {
				var rl *RateLimitError
				if !errors.As(err, &rl) || rl.RetryAfter != 30*time.Second {
					t.Errorf("error = %v, want RateLimitError with 30s", err)
				}
			},
		},
		{
			name:    "500 is a generic error",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) },
			check: func(t *testing.T, err error) {
				if err == nil || errors.Is(err, ErrNotFound) {
					t.Errorf("error = %v, want a non-nil, non-NotFound error", err)
				}
			},
		},
		{
			name: "invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"symbol":`))
			},
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Error("error = nil, want a decode error")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newClient(t, tt.handler).Quote(t.Context(), "ABC")
			tt.check(t, err)
		})
	}
}

// Count requests with an atomic: the handler runs in the server's goroutines.
func TestQuoteSendsOneRequest(t *testing.T) {
	var hits atomic.Int32
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{}`))
	}))
	if _, err := client.Quote(t.Context(), "ABC"); err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server received %d requests, want 1", got)
	}
}

// Cancellation is tested without sleeping: the handler announces that the
// request arrived, the test cancels, and the client must return promptly.
func TestQuoteContextCancelled(t *testing.T) {
	arrived := make(chan struct{})
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		select {
		case <-r.Context().Done(): // the client went away
		case <-release:
		}
	}))
	// Cleanups run last-in first-out: release the handler, then close the server
	// (Close waits for in-flight requests, so the handler must be able to finish).
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		_, err := client.Quote(ctx, "ABC")
		errc <- err
	}()

	<-arrived // the request is in flight
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

// NewTLSServer serves HTTPS with a throwaway certificate; srv.Client() is
// already configured to trust it.
func TestQuoteOverTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("request was not served over TLS")
		}
		w.Write([]byte(`{"symbol":"ABC"}`))
	}))
	t.Cleanup(srv.Close)

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	if q, err := client.Quote(t.Context(), "ABC"); err != nil || q.Symbol != "ABC" {
		t.Errorf("Quote = %+v, %v", q, err)
	}
}

// Without the server's client, the self-signed certificate is rejected.
// NewUnstartedServer lets us tweak the server before it starts; here we mute
// the "TLS handshake error" line the server would otherwise log.
func TestQuoteRejectsUntrustedCertificate(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)

	client := &Client{BaseURL: srv.URL} // http.DefaultClient does not trust it
	if _, err := client.Quote(t.Context(), "ABC"); err == nil {
		t.Error("Quote succeeded against an untrusted certificate")
	}
}
