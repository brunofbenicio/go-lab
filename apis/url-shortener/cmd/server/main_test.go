package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func envFrom(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config
		wantErr bool
	}{
		{name: "defaults", env: nil, want: config{Port: "8080"}},
		{name: "custom port", env: map[string]string{"PORT": "9090"}, want: config{Port: "9090"}},
		{name: "port zero asks the OS for a free port", env: map[string]string{"PORT": "0"}, want: config{Port: "0"}},
		{name: "highest port", env: map[string]string{"PORT": "65535"}, want: config{Port: "65535"}},
		{name: "base URL", env: map[string]string{"BASE_URL": "https://sho.example.com"}, want: config{Port: "8080", BaseURL: "https://sho.example.com"}},
		{name: "base URL with port", env: map[string]string{"BASE_URL": "http://localhost:3000"}, want: config{Port: "8080", BaseURL: "http://localhost:3000"}},
		{name: "port is not a number", env: map[string]string{"PORT": "http"}, wantErr: true},
		{name: "port is negative", env: map[string]string{"PORT": "-1"}, wantErr: true},
		{name: "port is too large", env: map[string]string{"PORT": "65536"}, wantErr: true},
		{name: "base URL without scheme", env: map[string]string{"BASE_URL": "sho.example.com"}, wantErr: true},
		{name: "base URL with unsupported scheme", env: map[string]string{"BASE_URL": "ftp://sho.example.com"}, wantErr: true},
		{name: "base URL without host", env: map[string]string{"BASE_URL": "https://"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configFromEnv(envFrom(tt.env))
			if (err != nil) != tt.wantErr {
				t.Fatalf("configFromEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("configFromEnv() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewHandlerServesDocs(t *testing.T) {
	handler, err := newHandler(config{Port: "8080"}, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/docs", "/openapi.yaml"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
	}
}

// waitUntilRefused polls addr until new connections are refused, which means
// the server stopped listening.
func waitUntilRefused(t *testing.T, addr string) {
	t.Helper()
	for range 3000 {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("server at %s kept accepting connections after shutdown began", addr)
}

func newTestClient(t *testing.T) *http.Client {
	t.Helper()
	tr := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

func TestServeShutsDownGracefully(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, "finished")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() { served <- serve(ctx, &http.Server{Handler: mux}, ln, 5*time.Second) }()

	type result struct {
		body string
		err  error
	}
	response := make(chan result, 1)
	go func() {
		resp, err := newTestClient(t).Get("http://" + addr + "/slow")
		if err != nil {
			response <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		response <- result{body: string(b), err: err}
	}()

	<-started // the request is now in flight
	cancel()  // ask the server to stop
	waitUntilRefused(t, addr)

	select {
	case err := <-served:
		t.Fatalf("serve returned (%v) while a request was still in flight", err)
	default:
	}

	close(release)
	if got := <-response; got.err != nil || got.body != "finished" {
		t.Errorf("in-flight request = %q, %v; want it to complete with %q", got.body, got.err, "finished")
	}
	if err := <-served; err != nil {
		t.Errorf("serve() = %v, want nil after a clean shutdown", err)
	}
}

func TestServeGivesUpAfterGracePeriod(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stuck", func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() { served <- serve(ctx, &http.Server{Handler: mux}, ln, 20*time.Millisecond) }()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := newTestClient(t).Get("http://" + ln.Addr().String() + "/stuck")
		if err == nil {
			resp.Body.Close()
		}
	}()

	<-started
	cancel()
	err = <-served
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("serve() = %v, want it to wrap context.DeadlineExceeded", err)
	}

	close(release)
	<-clientDone
}

func TestServeReturnsListenerFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close() // Serve will fail on the first Accept

	err = serve(context.Background(), &http.Server{Handler: http.NewServeMux()}, ln, time.Second)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Errorf("serve() = %v, want a listener error", err)
	}
}

func TestRun(t *testing.T) {
	t.Run("invalid configuration", func(t *testing.T) {
		err := run(context.Background(), envFrom(map[string]string{"PORT": "abc"}), discardLogger())
		if err == nil {
			t.Fatal("run() accepted an invalid PORT")
		}
	})

	t.Run("port already in use", func(t *testing.T) {
		busy, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer busy.Close()
		port := fmt.Sprint(busy.Addr().(*net.TCPAddr).Port)

		err = run(context.Background(), envFrom(map[string]string{"PORT": port}), discardLogger())
		if err == nil {
			t.Fatal("run() succeeded on a port that is already taken")
		}
	})

	t.Run("starts and stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := run(ctx, envFrom(map[string]string{"PORT": "0"}), discardLogger()); err != nil {
			t.Errorf("run() = %v, want nil", err)
		}
	})
}
