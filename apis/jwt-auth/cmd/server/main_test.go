package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/password"
)

// testSecret is an obviously fake key used only by these tests.
const testSecret = "test-only-secret-do-not-use-in-production-0123456789"

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// fastDeps makes password hashing cheap so tests do not pay for the production
// iteration count.
func fastDeps() deps {
	return deps{hasherOpts: []password.Option{password.WithIterations(10)}}
}

func envFrom(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config // Secret is compared separately
		wantErr string // substring of the expected error, empty when it must succeed
	}{
		{
			name: "defaults",
			env:  map[string]string{"JWT_SECRET": testSecret},
			want: config{Port: "8080", Issuer: "jwt-auth", TokenTTL: 15 * time.Minute},
		},
		{
			name: "everything customized",
			env: map[string]string{
				"JWT_SECRET": testSecret, "PORT": "9090", "JWT_ISSUER": "auth.example.com", "TOKEN_TTL": "2h",
			},
			want: config{Port: "9090", Issuer: "auth.example.com", TokenTTL: 2 * time.Hour},
		},
		{
			name: "port zero asks the OS for a free port",
			env:  map[string]string{"JWT_SECRET": testSecret, "PORT": "0"},
			want: config{Port: "0", Issuer: "jwt-auth", TokenTTL: 15 * time.Minute},
		},
		{
			name: "secret of exactly 32 bytes",
			env:  map[string]string{"JWT_SECRET": strings.Repeat("k", 32)},
			want: config{Port: "8080", Issuer: "jwt-auth", TokenTTL: 15 * time.Minute},
		},
		{
			name: "ttl of exactly 24 hours",
			env:  map[string]string{"JWT_SECRET": testSecret, "TOKEN_TTL": "24h"},
			want: config{Port: "8080", Issuer: "jwt-auth", TokenTTL: 24 * time.Hour},
		},
		{name: "missing secret", env: nil, wantErr: "JWT_SECRET is required"},
		{name: "secret one byte short", env: map[string]string{"JWT_SECRET": strings.Repeat("k", 31)}, wantErr: "too short"},
		{name: "port is not a number", env: map[string]string{"JWT_SECRET": testSecret, "PORT": "http"}, wantErr: "PORT"},
		{name: "port is negative", env: map[string]string{"JWT_SECRET": testSecret, "PORT": "-1"}, wantErr: "PORT"},
		{name: "port is too large", env: map[string]string{"JWT_SECRET": testSecret, "PORT": "65536"}, wantErr: "PORT"},
		{name: "ttl is not a duration", env: map[string]string{"JWT_SECRET": testSecret, "TOKEN_TTL": "soon"}, wantErr: "TOKEN_TTL"},
		{name: "ttl without a unit", env: map[string]string{"JWT_SECRET": testSecret, "TOKEN_TTL": "900"}, wantErr: "TOKEN_TTL"},
		{name: "ttl is zero", env: map[string]string{"JWT_SECRET": testSecret, "TOKEN_TTL": "0s"}, wantErr: "TOKEN_TTL"},
		{name: "ttl is negative", env: map[string]string{"JWT_SECRET": testSecret, "TOKEN_TTL": "-5m"}, wantErr: "TOKEN_TTL"},
		{name: "ttl above 24 hours", env: map[string]string{"JWT_SECRET": testSecret, "TOKEN_TTL": "25h"}, wantErr: "TOKEN_TTL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configFromEnv(envFrom(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("configFromEnv() error = %v, want it to contain %q", err, tt.wantErr)
				}
				if secret := tt.env["JWT_SECRET"]; secret != "" && strings.Contains(err.Error(), secret) {
					t.Errorf("the error message leaks the secret: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("configFromEnv() failed: %v", err)
			}
			if got.Port != tt.want.Port || got.Issuer != tt.want.Issuer || got.TokenTTL != tt.want.TokenTTL {
				t.Errorf("configFromEnv() = %+v, want %+v", got, tt.want)
			}
			if string(got.Secret) != tt.env["JWT_SECRET"] {
				t.Error("the secret was not taken from JWT_SECRET")
			}
		})
	}
}

func TestConfigLogValueHidesTheSecret(t *testing.T) {
	cfg, err := configFromEnv(envFrom(map[string]string{"JWT_SECRET": testSecret}))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	slog.New(slog.NewTextHandler(&out, nil)).Info("starting", "config", cfg)

	if strings.Contains(out.String(), testSecret) {
		t.Errorf("the secret was logged: %s", out.String())
	}
	if !strings.Contains(out.String(), "secret_set=true") {
		t.Errorf("log does not say that a secret is set: %s", out.String())
	}
}

func TestNewHandlerServesDocs(t *testing.T) {
	cfg, err := configFromEnv(envFrom(map[string]string{"JWT_SECRET": testSecret}))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(cfg, discardLogger(), fastDeps())
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

func TestNewHandlerRejectsAWeakSecret(t *testing.T) {
	if _, err := newHandler(config{Secret: []byte("short")}, discardLogger(), deps{}); err == nil {
		t.Error("newHandler() accepted a weak secret")
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
	t.Run("missing secret", func(t *testing.T) {
		err := run(context.Background(), envFrom(nil), discardLogger(), fastDeps())
		if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
			t.Errorf("run() error = %v, want a complaint about JWT_SECRET", err)
		}
	})

	t.Run("port already in use", func(t *testing.T) {
		busy, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer busy.Close()
		port := fmt.Sprint(busy.Addr().(*net.TCPAddr).Port)

		err = run(context.Background(), envFrom(map[string]string{"JWT_SECRET": testSecret, "PORT": port}), discardLogger(), fastDeps())
		if err == nil {
			t.Fatal("run() succeeded on a port that is already taken")
		}
	})

	t.Run("starts and stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		if err := run(ctx, envFrom(map[string]string{"JWT_SECRET": testSecret, "PORT": "0"}), logger, fastDeps()); err != nil {
			t.Errorf("run() = %v, want nil", err)
		}
		if strings.Contains(logs.String(), testSecret) {
			t.Errorf("the secret was logged at startup: %s", logs.String())
		}
	})
}
