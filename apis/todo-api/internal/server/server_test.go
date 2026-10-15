package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func TestRunFinishesInFlightRequestsBeforeStopping(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	})}
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() { runDone <- Run(ctx, srv, ln, 5*time.Second) }()

	type result struct {
		body string
		err  error
	}
	clientDone := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err != nil {
			clientDone <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		clientDone <- result{body: string(body), err: err}
	}()

	<-started
	cancel() // the shutdown signal arrives while the request is being served

	select {
	case err := <-runDone:
		t.Fatalf("Run returned %v while a request was still in flight", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	res := <-clientDone
	if res.err != nil || res.body != "finished" {
		t.Errorf("in-flight request got body %q and error %v, want it to complete", res.body, res.err)
	}
	if err := <-runDone; err != nil {
		t.Errorf("Run() = %v, want nil after a clean shutdown", err)
	}

	if _, err := http.Get("http://" + ln.Addr().String()); err == nil {
		t.Error("the listener should be closed after shutdown")
	}
}

func TestRunCutsOffRequestsThatOutliveTheTimeout(t *testing.T) {
	started := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done() // only returns once the connection is closed
	})}
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() { runDone <- Run(ctx, srv, ln, 50*time.Millisecond) }()

	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-runDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Run() = %v, want an error wrapping context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not give up on the stuck request")
	}
}

func TestRunReturnsServeErrors(t *testing.T) {
	ln := listen(t)
	ln.Close() // serving on a closed listener fails immediately
	srv := &http.Server{Handler: http.NotFoundHandler()}

	err := Run(context.Background(), srv, ln, time.Second)

	if err == nil {
		t.Fatal("Run() = nil, want the serve error")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run() = %v, a real failure must not look like a clean stop", err)
	}
}

func TestRunStopsPromptlyWhenIdle(t *testing.T) {
	srv := &http.Server{Handler: http.NotFoundHandler()}
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() { runDone <- Run(ctx, srv, ln, 5*time.Second) }()
	cancel()

	select {
	case err := <-runDone:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("an idle server should stop immediately")
	}
}
